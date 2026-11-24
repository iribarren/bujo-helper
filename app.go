package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"bujo-helper/internal/batch"
	"bujo-helper/internal/buildinfo"
	"bujo-helper/internal/catalog"
	"bujo-helper/internal/downloader"
	"bujo-helper/internal/fonts"
	"bujo-helper/internal/gallery"
	"bujo-helper/internal/i18n"
	"bujo-helper/internal/media"
	"bujo-helper/internal/preview"
	"bujo-helper/internal/quotes"
	"bujo-helper/internal/settings"
	"bujo-helper/internal/sheet"
	"bujo-helper/internal/web"
)

// Events emitted to the frontend while a job runs.
const (
	EventItem = "job:item" // one ItemEvent per finished item
	EventDone = "job:done" // one DoneEvent when the job ends

	// EventRateLimitWait is sent with a RateLimitEvent when a source
	// rate-limits and the client waits before retrying.
	EventRateLimitWait = "ratelimit:wait"
)

// RateLimitEvent is the payload of EventRateLimitWait. Seconds is the wait,
// rounded up to a whole second.
type RateLimitEvent struct {
	Host    string `json:"host"`
	Seconds int    `json:"seconds"`
}

// MediaTypeInfo describes a media type for the media type selector.
type MediaTypeInfo struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	CreatorLabel string `json:"creatorLabel"`
	DateLabel    string `json:"dateLabel"` // date field label on review cards
	// Sources are the display names of the type's providers, in the order
	// they are tried.
	Sources []string `json:"sources"`
}

// defaultCountry is used when the UI sends no country: Spain, so covers of
// Spanish editions are preferred.
const defaultCountry = "es"

// StartRequest is what the UI sends to start a job.
type StartRequest struct {
	MediaType string `json:"mediaType"`
	Text      string `json:"text"`
	OutputDir string `json:"outputDir"`
	Country   string `json:"country"`
}

// ItemView is one parsed item, rendered as a pending row before results
// arrive.
type ItemView struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Creator string `json:"creator"`
}

// ItemEvent is the payload of EventItem.
type ItemEvent struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Creator string `json:"creator"`
	Status  string `json:"status"`
	Path    string `json:"path"`
	Source  string `json:"source"`
	Matched string `json:"matched"`
	// Error is the English detail of a failure. Code is a stable failure
	// code the UI can show its own message for: "" (success or a failure
	// with no specific code), "not_found", "cancelled", "empty_title" or
	// "no_providers".
	Error string `json:"error"`
	Code  string `json:"code"`
}

// DoneEvent is the payload of EventDone. Error is set when the job could
// not run at all.
type DoneEvent struct {
	OK        int    `json:"ok"`
	Failed    int    `json:"failed"`
	Cancelled int    `json:"cancelled"`
	Error     string `json:"error,omitempty"`
	// Code is the stable code of Error, like ItemEvent.Code.
	Code string `json:"code,omitempty"`
}

// App is the Go side of the UI. Its exported methods are bound to the
// frontend by Wails.
type App struct {
	ctx        context.Context
	registry   *media.Registry
	downloader downloader.Downloader

	// emit sends an event to the frontend. Nil means runtime.EventsEmit;
	// tests replace it to capture events.
	emit func(event string, data any)

	mu     sync.Mutex
	cancel context.CancelFunc // non-nil while a job is running

	// thumbs serves the thumbnails of the covers listed by ListCovers. It
	// is the Wails asset server's fallback handler (see main.go).
	thumbs *gallery.Thumbs

	// previews holds the PDFs rendered by PreviewPDF and PreviewPage for
	// the live preview; served next to thumbs (see assetHandler).
	previews *preview.Store

	// quotes searches Wikiquote for SearchQuotes; tests point it at a
	// local server.
	quotes *quotes.Client

	// settingsPath is the settings file; "" means it is unknown and
	// SaveSettings fails. settings is the current copy, read live by
	// tmdbKey, igdbCredentials and bhlKey so saved keys are used without a
	// restart.
	settingsMu   sync.Mutex
	settingsPath string
	settings     settings.Settings
	// firstRun is true until FirstRun reports it: no settings file existed
	// when loadSettings ran.
	firstRun bool

	// lang is the UI language of dialogs, errors and PDF labels; the
	// frontend sets it with SetLanguage. The zero value means English.
	langMu sync.Mutex
	lang   i18n.Lang
}

// SetLanguage sets the UI language of native dialogs, errors and PDF
// labels, from a language tag such as "es" or "es-ES". An unsupported one
// gives English.
func (a *App) SetLanguage(lang string) {
	a.langMu.Lock()
	defer a.langMu.Unlock()
	a.lang = i18n.Parse(lang)
}

// language returns the current UI language.
func (a *App) language() i18n.Lang {
	a.langMu.Lock()
	defer a.langMu.Unlock()
	if a.lang == "" {
		return i18n.EN
	}
	return a.lang
}

// t translates key into the current UI language.
func (a *App) t(key string, args ...any) string {
	return i18n.T(a.language(), key, args...)
}

// NewApp returns an App with the built-in media types and the user's
// settings.
func NewApp() *App {
	path, err := settings.Path()
	if err != nil {
		log.Printf("settings: %v", err)
	} else {
		migrateSettings(path)
	}
	return newDefaultApp(web.NewClient(), path)
}

// migrateSettings copies the settings saved before the rename from Cover
// Downloader to BuJo Helper, when path does not exist yet. A skipped
// legacy file or a failed copy is logged: the app still starts, only
// without the old keys.
func migrateSettings(path string) {
	legacy, err := settings.LegacyPath()
	if err != nil {
		log.Printf("settings: %v", err)
		return
	}
	err = settings.Migrate(legacy, path)
	switch {
	case errors.Is(err, settings.ErrLegacySkipped):
		log.Printf("settings: %v; starting without the old settings", err)
	case err != nil:
		log.Printf("settings: %v", err)
	}
}

// newDefaultApp returns an App with the built-in media types and the
// settings file at path. The movie, TV, game, nature and board game sources
// read their keys from the App's live settings; BoardGameGeek falls back to
// the built-in token.
func newDefaultApp(client *http.Client, path string) *App {
	var a *App
	// The catalog is built before the App exists; a is set before any
	// search can call the key func.
	reg := catalog.Default(client, catalog.Keys{
		TMDB: func() string { return a.tmdbKey() },
		IGDB: func() (string, string) { return a.igdbCredentials() },
		BHL:  func() string { return a.bhlKey() },
		BGG:  func() string { return a.bggToken() },
	})
	a = newApp(reg, client)
	a.loadSettings(path)
	return a
}

func newApp(reg *media.Registry, client *http.Client) *App {
	return &App{
		registry:   reg,
		downloader: downloader.Downloader{Registry: reg, Client: client},
		thumbs:     gallery.NewThumbs(),
		previews:   preview.NewStore(),
		quotes:     quotes.NewClient(client),
	}
}

// assetHandler serves what the Wails asset server cannot find among the
// embedded assets: thumbnails, preview PDFs and the lettering fonts.
// Anything else is 404.
func (a *App) assetHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(gallery.ThumbPath, a.thumbs)
	mux.Handle(preview.Path, a.previews)
	mux.Handle(fonts.Path, fonts.Handler{})
	return mux
}

// loadSettings reads the settings file at path and remembers path for
// SaveSettings, and whether the file was missing for FirstRun. A read error
// is logged and leaves the settings empty: the app still works, only
// without the saved keys.
func (a *App) loadSettings(path string) {
	var s settings.Settings
	first := false
	if path != "" {
		_, err := os.Stat(path)
		first = errors.Is(err, os.ErrNotExist)
		if s, err = settings.Load(path); err != nil {
			log.Printf("settings: %v", err)
		}
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	a.settingsPath = path
	a.settings = s
	a.firstRun = first
}

// FirstRun reports whether this is the app's first run on this computer:
// no settings file existed at startup. It reports true once: that call
// writes the settings file, so later calls and later runs report false. A
// failed write is logged; FirstRun still reports true this once.
func (a *App) FirstRun() bool {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if !a.firstRun {
		return false
	}
	a.firstRun = false
	if err := a.saveSettingsLocked(a.settings); err != nil {
		log.Printf("settings: %v", err)
	}
	return true
}

// GetSettings returns the current settings.
func (a *App) GetSettings() settings.Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings
}

// SaveSettings saves s to the settings file and uses it at once. The
// remembered Print covers and Make pages options are kept: the Settings
// dialog does not send them. It sends everything else, the theme and the
// shown parts of the app included.
func (a *App) SaveSettings(s settings.Settings) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	s.Print = a.settings.Print
	s.Pages = a.settings.Pages
	return a.saveSettingsLocked(s)
}

// SavePrintPrefs remembers the Print covers options, keeping every other
// setting.
func (a *App) SavePrintPrefs(p settings.PrintPrefs) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	s := a.settings
	s.Print = p
	return a.saveSettingsLocked(s)
}

// SavePagesPrefs remembers the Make pages kind and shared options, keeping
// every other setting.
func (a *App) SavePagesPrefs(p settings.PagesPrefs) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	s := a.settings
	s.Pages = p
	return a.saveSettingsLocked(s)
}

// saveSettingsLocked trims s, writes it to the settings file and uses it
// at once. The caller holds settingsMu.
func (a *App) saveSettingsLocked(s settings.Settings) error {
	s = s.Trimmed()
	if a.settingsPath == "" {
		return errors.New(a.t("err.settingsLocation"))
	}
	if err := settings.Save(a.settingsPath, s); err != nil {
		return err
	}
	a.settings = s
	return nil
}

// tmdbKey returns the current TMDB API key. It reads the settings live, so
// a key saved in the UI applies to the next search.
func (a *App) tmdbKey() string {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings.TMDBAPIKey
}

// igdbCredentials returns the current Twitch Client ID and Client Secret
// for IGDB, read live like tmdbKey.
func (a *App) igdbCredentials() (clientID, clientSecret string) {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings.IGDBClientID, a.settings.IGDBClientSecret
}

// bhlKey returns the current Biodiversity Heritage Library API key, read
// live like tmdbKey.
func (a *App) bhlKey() string {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings.BHLAPIKey
}

// bggToken returns the BoardGameGeek token: the one saved in the settings,
// read live like tmdbKey, else the one built into this build (empty in
// source builds).
func (a *App) bggToken() string {
	a.settingsMu.Lock()
	saved := strings.TrimSpace(a.settings.BGGToken)
	a.settingsMu.Unlock()
	if saved != "" {
		return saved
	}
	return buildinfo.BGGToken
}

// HasBuiltInBGGToken reports whether this build has a BoardGameGeek token
// built in, so board games work without one in the settings.
func (*App) HasBuiltInBGGToken() bool {
	return strings.TrimSpace(buildinfo.BGGToken) != ""
}

// startup is called when the app starts. The context is kept for the
// runtime calls (dialogs, events).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	web.DefaultThrottle.SetOnWait(a.onRateLimitWait)
}

// shutdown cancels a running job when the window closes and removes the
// rate-limit hook.
func (a *App) shutdown(context.Context) {
	a.Cancel()
	web.DefaultThrottle.SetOnWait(nil)
}

// onRateLimitWait tells the UI that requests to host wait d before they
// retry. A wait of zero or less sends nothing.
func (a *App) onRateLimitWait(host string, d time.Duration) {
	if d <= 0 {
		return
	}
	seconds := int((d + time.Second - 1) / time.Second)
	a.send(EventRateLimitWait, RateLimitEvent{Host: host, Seconds: seconds})
}

func (a *App) send(event string, data any) {
	if a.emit != nil {
		a.emit(event, data)
		return
	}
	runtime.EventsEmit(a.ctx, event, data)
}

// MediaTypes lists the registered media types in registration order.
func (a *App) MediaTypes() []MediaTypeInfo {
	var out []MediaTypeInfo
	for _, t := range a.registry.List() {
		out = append(out, MediaTypeInfo{
			ID:           t.ID,
			Label:        t.Label,
			CreatorLabel: t.CreatorLabel,
			DateLabel:    t.DateLabel,
			Sources:      sources(t.Providers),
		})
	}
	return out
}

// sources returns the display names of providers, in order.
func sources(providers []media.Provider) []string {
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, media.ProviderLabel(p))
	}
	return out
}

// DefaultOutputDir is <home>/Pictures/Covers, or "Covers" in the working
// directory when the home directory is unknown.
func (a *App) DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Covers"
	}
	return filepath.Join(home, "Pictures", "Covers")
}

// ImportCSV asks for a CSV file and returns its items as "Title, Creator"
// lines. It returns "" with no error when the dialog is cancelled.
func (a *App) ImportCSV() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: a.t("dialog.importCSV"),
		Filters: []runtime.FileFilter{
			{DisplayName: a.t("filter.csv"), Pattern: "*.csv"},
			{DisplayName: a.t("filter.all"), Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	return importCSVFile(path, a.language())
}

// importCSVFile reads the items of the CSV file at path as "Title, Creator"
// lines. A file with no items is an error, in lang.
func importCSVFile(path string, lang i18n.Lang) (string, error) {
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	items, err := batch.ReadCSV(f)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	if len(items) == 0 {
		return "", errors.New(i18n.T(lang, "err.noItemsInFile", name))
	}
	return batch.FormatLines(items), nil
}

// The native dialogs, as variables so tests can replace them.
var (
	openFileDialog = runtime.OpenFileDialog
	saveFileDialog = runtime.SaveFileDialog
	messageDialog  = runtime.MessageDialog
)

// ImportRecipes asks for a Markdown file and returns its recipes, as
// sheet.ParseRecipesMarkdown reads them. It returns an empty list (JSON
// []) with no error when the dialog is cancelled.
func (a *App) ImportRecipes() ([]sheet.Recipe, error) {
	path, err := openFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: a.t("dialog.importRecipes"),
		Filters: []runtime.FileFilter{
			{DisplayName: a.t("filter.markdown"), Pattern: "*.md;*.markdown;*.txt"},
			{DisplayName: a.t("filter.all"), Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return []sheet.Recipe{}, nil
	}
	return importRecipesFile(path, a.language())
}

// importRecipesFile reads the recipes of the Markdown file at path, photo
// paths relative to its folder. A file with no recipes is an error, in
// lang; other errors are prefixed with the file name.
func importRecipesFile(path string, lang i18n.Lang) ([]sheet.Recipe, error) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	recipes, err := sheet.ParseRecipesMarkdown(f, filepath.Dir(path))
	if errors.Is(err, sheet.ErrNoRecipes) {
		return nil, errors.New(i18n.T(lang, "err.noRecipesInFile", name))
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return recipes, nil
}

// ChooseRecipePhoto asks for a JPEG or PNG image and returns its absolute
// path, or "" with no error when the dialog is cancelled.
func (a *App) ChooseRecipePhoto() (string, error) {
	path, err := openFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: a.t("dialog.recipePhoto"),
		Filters: []runtime.FileFilter{
			{DisplayName: a.t("filter.images"), Pattern: "*.jpg;*.jpeg;*.png"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	return filepath.Abs(path)
}

// SaveSampleCSV asks where to save a sample CSV for the media type with the
// given ID, which the user can use as a reference for Import CSV. It returns
// the saved path, or "" with no error when the dialog is cancelled.
func (a *App) SaveSampleCSV(mediaType string) (string, error) {
	name, data, err := a.sampleCSV(mediaType)
	if err != nil {
		return "", err
	}
	path, err := saveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                a.t("dialog.saveSampleCSV"),
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: a.t("filter.csv"), Pattern: "*.csv"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	if path, err = a.withExtension(path, ".csv"); err != nil || path == "" {
		return "", err
	}
	if err := writeFileAtomic(path, "CSV", func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}); err != nil {
		return "", err
	}
	return path, nil
}

// sampleCSV returns the file name and contents of the sample CSV for a
// media type: a "title,<creator>" header and its sample items, or a
// "title" column alone for a type without a creator column (nature). It
// starts with a UTF-8 BOM so Excel on Windows reads accented names
// correctly.
func (a *App) sampleCSV(mediaType string) (string, []byte, error) {
	t, ok := a.registry.Get(mediaType)
	if !ok {
		return "", nil, errors.New(a.t("err.unknownMediaType", mediaType))
	}
	var b bytes.Buffer
	b.WriteString("\xef\xbb\xbf")
	if t.CreatorLabel == "" {
		if err := writeTitlesCSV(&b, t.Samples); err != nil {
			return "", nil, err
		}
		return "sample_" + t.ID + ".csv", b.Bytes(), nil
	}
	header := []string{"title", strings.ToLower(t.CreatorLabel)}
	if err := batch.WriteCSV(&b, header, t.Samples); err != nil {
		return "", nil, err
	}
	return "sample_" + t.ID + ".csv", b.Bytes(), nil
}

// writeTitlesCSV writes a one-column CSV: a "title" header and each item's
// title. It reads back with batch.ReadCSV like a two-column one.
func writeTitlesCSV(w io.Writer, items []media.Item) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"title"}); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	for _, it := range items {
		if err := cw.Write([]string{it.Title}); err != nil {
			return fmt.Errorf("write csv: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	return nil
}

// ChooseFolder asks for an output folder, starting at current. It returns
// "" with no error when the dialog is cancelled.
func (a *App) ChooseFolder(current string) (string, error) {
	opts := runtime.OpenDialogOptions{Title: a.t("dialog.chooseOutputFolder"), CanCreateDirectories: true}
	if info, err := os.Stat(current); err == nil && info.IsDir() {
		opts.DefaultDirectory = current
	}
	return runtime.OpenDirectoryDialog(a.ctx, opts)
}

// Start parses req.Text and downloads the covers in the background. It
// returns the parsed items at once; results arrive as EventItem events and
// the job ends with one EventDone event.
func (a *App) Start(req StartRequest) ([]ItemView, error) {
	items := batch.ParseLines(req.Text)
	if len(items) == 0 {
		return nil, errors.New(a.t("err.noItems"))
	}
	if _, ok := a.registry.Get(req.MediaType); !ok {
		return nil, errors.New(a.t("err.unknownMediaType", req.MediaType))
	}
	outDir := strings.TrimSpace(req.OutputDir)
	if outDir == "" {
		return nil, errors.New(a.t("err.chooseOutputFolder"))
	}
	country := strings.ToLower(strings.TrimSpace(req.Country))
	if country == "" {
		country = defaultCountry
	}

	// Take the running slot before touching the disk, so a Start refused
	// for a running job creates no folder; give it back if the folder
	// cannot be made.
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return nil, errors.New(a.t("err.downloadRunning"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.mu.Unlock()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("%s: %w", a.t("err.createOutputFolder"), err)
	}

	views := make([]ItemView, len(items))
	for i, it := range items {
		views[i] = ItemView{Index: i, Title: it.Title, Creator: it.Creator}
	}
	go a.run(ctx, downloader.Request{
		MediaType: req.MediaType,
		Items:     items,
		OutputDir: outDir,
		Options:   media.Options{Country: country},
	})
	return views, nil
}

func (a *App) run(ctx context.Context, req downloader.Request) {
	sum, err := a.downloader.Run(ctx, req, func(r downloader.ItemResult) {
		a.send(EventItem, ItemEvent{
			Index:   r.Index,
			Title:   r.Item.Title,
			Creator: r.Item.Creator,
			Status:  r.Status,
			Path:    r.Path,
			Source:  r.Source,
			Matched: r.Matched,
			Error:   r.Error,
			Code:    downloader.Code(r.Err),
		})
	})
	done := DoneEvent{OK: sum.OK, Failed: sum.Failed, Cancelled: sum.Cancelled}
	if err != nil {
		done.Error, done.Code = err.Error(), downloader.Code(err)
	}

	// Clear the running state before announcing the end, so the UI can
	// start the next job as soon as it sees EventDone.
	a.mu.Lock()
	a.cancel()
	a.cancel = nil
	a.mu.Unlock()
	a.send(EventDone, done)
}

// Cancel stops the running job, if any. Unfinished items are reported as
// cancelled.
func (a *App) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// OpenFolder opens path in the system file manager, creating it first.
func (a *App) OpenFolder(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New(a.t("err.noFolderToOpen"))
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	// explorer exits with status 1 even on success, so the exit status is
	// ignored; Wait only reaps the process.
	go cmd.Wait()
	return nil
}

// CoverView is one image of the print sheet gallery.
type CoverView struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	ThumbURL string `json:"thumbUrl"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// ListCovers returns the images in dir, sorted by name, and makes their
// thumbnails available to the UI.
func (a *App) ListCovers(dir string) ([]CoverView, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New(a.t("err.chooseFolder"))
	}
	covers, err := gallery.List(dir)
	if err != nil {
		return nil, err
	}
	a.thumbs.Allow(covers)
	views := make([]CoverView, len(covers))
	for i, c := range covers {
		views[i] = CoverView{
			Name:     c.Name,
			Path:     c.Path,
			ThumbURL: gallery.URL(c.Path),
			Width:    c.Width,
			Height:   c.Height,
		}
	}
	return views, nil
}

// PrintFormatInfo describes a print format for the Print sheet tab.
type PrintFormatInfo struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Noun       string `json:"noun"`       // what one printed item is called, e.g. "card"
	UsesSize   bool   `json:"usesSize"`   // whether the cover size preset applies
	UsesCut    bool   `json:"usesCut"`    // whether the cut mode applies
	UsesCrop   bool   `json:"usesCrop"`   // whether the crop mode applies
	UsesHeader bool   `json:"usesHeader"` // whether the format prints the header the user types
	// UsesLettering is whether the format prints display text in the
	// chosen lettering font and Colour-in mode.
	UsesLettering bool `json:"usesLettering"`
	Default       bool `json:"default"`
}

// PrintFormats lists the print formats, the default first.
func (a *App) PrintFormats() []PrintFormatInfo {
	out := make([]PrintFormatInfo, 0, len(sheet.Formats))
	for _, f := range sheet.Formats {
		out = append(out, PrintFormatInfo{
			ID:            f.ID,
			Label:         f.Label,
			Noun:          f.Noun,
			UsesSize:      f.UsesSize,
			UsesCut:       f.UsesCut,
			UsesCrop:      f.UsesCrop,
			UsesHeader:    f.UsesHeader,
			UsesLettering: f.UsesLettering,
			Default:       f.ID == sheet.DefaultFormat,
		})
	}
	return out
}

// LetteringFontInfo describes a lettering font for the font picker.
type LetteringFontInfo struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Group      string `json:"group"`  // style group: sans, brush, monoline, handwritten, display, hollow
	Hollow     string `json:"hollow"` // hollow letters: stroke, none or native
	Licence    string `json:"licence"`
	URL        string `json:"url"`        // the TTF, for @font-face
	LicenceURL string `json:"licenceUrl"` // the licence text
	Default    bool   `json:"default"`
}

// LetteringFonts lists the lettering fonts, the default first.
func (a *App) LetteringFonts() []LetteringFontInfo {
	out := make([]LetteringFontInfo, 0, len(sheet.LetteringFonts))
	for _, f := range sheet.LetteringFonts {
		out = append(out, LetteringFontInfo{
			ID:         f.ID,
			Label:      f.Label,
			Group:      f.Group,
			Hollow:     string(f.Hollow),
			Licence:    f.Licence,
			URL:        fonts.TTFURL(f.ID),
			LicenceURL: fonts.LicenceURL(f.ID),
			Default:    f.ID == sheet.DefaultLetteringFont,
		})
	}
	return out
}

// PrintCutModeInfo describes a cut mode for the Print sheet tab.
type PrintCutModeInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Default bool   `json:"default"`
}

// PrintCutModes lists the cut modes of a cover sheet, the default first.
// They never change how many covers fit on a page.
func (a *App) PrintCutModes() []PrintCutModeInfo {
	out := make([]PrintCutModeInfo, 0, len(sheet.CutModes))
	for _, c := range sheet.CutModes {
		out = append(out, PrintCutModeInfo{
			ID:      c.ID,
			Label:   c.Label,
			Default: c.ID == sheet.DefaultCut,
		})
	}
	return out
}

// PrintCropModeInfo describes a crop mode for the Print sheet tab.
type PrintCropModeInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	UsesSize bool   `json:"usesSize"` // whether the cover size preset applies
	UsesCut  bool   `json:"usesCut"`  // whether the cut mode applies
	Frames   int    `json:"frames"`   // photos per printed piece: a strip's frames, else 1
	Default  bool   `json:"default"`
}

// PrintCropModes lists the crop modes of a cover sheet, the default (no
// cropping) first. PrintPapers has how many photos each fits on a page.
func (a *App) PrintCropModes() []PrintCropModeInfo {
	out := make([]PrintCropModeInfo, 0, len(sheet.CropModes))
	for _, c := range sheet.CropModes {
		out = append(out, PrintCropModeInfo{
			ID:       c.ID,
			Label:    c.Label,
			UsesSize: c.UsesSize,
			UsesCut:  c.UsesCut,
			Frames:   c.Frames,
			Default:  c.ID == sheet.DefaultCrop,
		})
	}
	return out
}

// PrintSizeInfo describes a cover size preset for the Print sheet tab.
type PrintSizeInfo struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	CoverHeight float64 `json:"coverHeight"` // mm
	Default     bool    `json:"default"`
}

// PrintSizes lists the cover size presets, smallest first. How many items
// fit on a page depends on the paper too; PrintPapers has those counts.
func (a *App) PrintSizes() []PrintSizeInfo {
	out := make([]PrintSizeInfo, 0, len(sheet.Sizes))
	for _, s := range sheet.Sizes {
		out = append(out, PrintSizeInfo{
			ID:          s.ID,
			Label:       s.Label,
			CoverHeight: s.CoverHeight,
			Default:     s.ID == sheet.DefaultSize,
		})
	}
	return out
}

// PrintPaperInfo describes a paper size preset for the Print sheet tab.
type PrintPaperInfo struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Width   float64 `json:"width"`  // mm
	Height  float64 `json:"height"` // mm
	Default bool    `json:"default"`
	// PerPage is what fits on a full page, by format ID, then size ID; a
	// format that does not use a cover size has one count, under "". For
	// example 25 standard covers, or 8 review cards, on A4.
	PerPage map[string]map[string]int `json:"perPage"`
	// HeaderPerPage is what fits on a full page with a header, by format
	// ID, for the formats that print one; PerPage has their counts without
	// it. For example 16 checklist rows on A4, against 17 without a header.
	HeaderPerPage map[string]int `json:"headerPerPage"`
	// SquarePerPage is how many square images fit on a full page, by
	// format ID, then size ID, for the formats that give them square cells
	// (the cover sheet); in other formats they fit PerPage. For example 15
	// standard album covers on A4, against 25 book covers.
	SquarePerPage map[string]map[string]int `json:"squarePerPage"`
	// CropPerPage is how many photos fit on a full page of a cover sheet
	// cropped to a standard print shape, by crop mode ID, then size ID for
	// a mode that uses one (else ""), for every crop mode but None (which
	// is PerPage). For example 9 instant minis, or 12 photos in strips of
	// 4, on A4.
	CropPerPage map[string]map[string]int `json:"cropPerPage"`
}

// PrintPapers lists the paper size presets, the default first, with how many
// items of each format (and size, if it uses one; and with or without a
// header, if it prints one) fit on a page, so the UI can count pages without
// its own copy of the layout rules.
func (a *App) PrintPapers() []PrintPaperInfo {
	out := make([]PrintPaperInfo, 0, len(sheet.Papers))
	for _, p := range sheet.Papers {
		perPage := make(map[string]map[string]int, len(sheet.Formats))
		headerPerPage := make(map[string]int)
		squarePerPage := make(map[string]map[string]int)
		for _, f := range sheet.Formats {
			if f.UsesHeader {
				// Any header takes the same band.
				n, err := sheet.Options{Format: f.ID, Paper: p.ID, Header: "Header"}.PerPage()
				if err != nil {
					log.Printf("print format %s with a header on %s: %v", f.ID, p.ID, err)
				} else {
					headerPerPage[f.ID] = n
				}
			}
			sizes := []string{""}
			if f.UsesSize {
				sizes = make([]string, 0, len(sheet.Sizes))
				for _, s := range sheet.Sizes {
					sizes = append(sizes, s.ID)
				}
			}
			counts := make(map[string]int, len(sizes))
			squares := make(map[string]int, len(sizes))
			for _, size := range sizes {
				opts := sheet.Options{Format: f.ID, Size: size, Paper: p.ID}
				n, err := opts.PerPage()
				if err != nil {
					// Every format and size fits every paper; sheet tests
					// guard this.
					log.Printf("print format %s, size %q on %s: %v", f.ID, size, p.ID, err)
					continue
				}
				counts[size] = n
				if f.ShapedCells {
					if n, err := opts.SquarePerPage(); err != nil {
						log.Printf("square images in print format %s, size %q on %s: %v", f.ID, size, p.ID, err)
					} else {
						squares[size] = n
					}
				}
			}
			perPage[f.ID] = counts
			if f.ShapedCells {
				squarePerPage[f.ID] = squares
			}
		}
		out = append(out, PrintPaperInfo{
			ID:            p.ID,
			Label:         p.Label,
			Width:         p.Page.Width,
			Height:        p.Page.Height,
			Default:       p.ID == sheet.DefaultPaper,
			PerPage:       perPage,
			HeaderPerPage: headerPerPage,
			SquarePerPage: squarePerPage,
			CropPerPage:   cropPerPage(p.ID),
		})
	}
	return out
}

// cropPerPage is PrintPaperInfo.CropPerPage for the paper paperID.
func cropPerPage(paperID string) map[string]map[string]int {
	out := make(map[string]map[string]int, len(sheet.CropModes))
	for _, c := range sheet.CropModes {
		if c.ID == sheet.DefaultCrop {
			continue
		}
		sizes := []string{""}
		if c.UsesSize {
			sizes = make([]string, 0, len(sheet.Sizes))
			for _, s := range sheet.Sizes {
				sizes = append(sizes, s.ID)
			}
		}
		counts := make(map[string]int, len(sizes))
		for _, size := range sizes {
			n, err := sheet.Options{Paper: paperID, Size: size, Crop: c.ID}.PerPage()
			if err != nil {
				// Every crop mode fits every paper; sheet tests guard
				// this.
				log.Printf("crop mode %s, size %q on %s: %v", c.ID, size, paperID, err)
				continue
			}
			counts[size] = n
		}
		out[c.ID] = counts
	}
	return out
}

// PrintPagesInfo is the page count of the images selected in the Print
// sheet tab.
type PrintPagesInfo struct {
	Pages int `json:"pages"`
	// AllSquare is whether there are images and every one is square, so a
	// cover sheet gives each a square cell (see PrintPaperInfo.SquarePerPage).
	AllSquare bool `json:"allSquare"`
	// Exact is false when an image could not be read: Pages then counts it
	// as a 2:3 cover, AllSquare is false and WidePerPage is empty.
	Exact bool `json:"exact"`
	// WidePerPage is how many images shaped like the selection fit on a
	// full page of the chosen paper, by size ID, for each size at which
	// every image gets a wide cell on the cover sheet (landscape images and
	// 4:5 portraits; see sheet.Grid.Wide). The shape is the selection's
	// middle aspect ratio, so for one shape the count is exact. Other sizes,
	// other formats, and selections with a square or 2:3 image are not
	// listed: the UI shows the square or 2:3 count instead.
	WidePerPage map[string]int `json:"widePerPage"`
}

// PrintPages counts the pages the images at paths take with the print
// settings opts, reading their image headers: on a cover sheet, square
// and landscape images get square and wide cells, so fewer fit on a page.
// With opts.Credits, the count includes the credits pages.
// An image that cannot be read counts as a 2:3 cover, so the toolbar still
// shows a count; ExportPDF reports the bad file.
func (a *App) PrintPages(paths []string, opts sheet.Options) (PrintPagesInfo, error) {
	if _, err := opts.Grid(); err != nil {
		return PrintPagesInfo{}, err
	}
	exact := true
	ratios := make([]float64, len(paths))
	for i, p := range paths {
		r, err := sheet.AspectRatios([]string{p})
		if err != nil {
			log.Printf("count print pages: %v", err)
			exact = false
			ratios[i] = 2.0 / 3
			continue
		}
		ratios[i] = r[0]
	}
	pages, err := opts.Pages(ratios)
	if err != nil {
		return PrintPagesInfo{}, err
	}
	// The credits pages follow the images; ExportPDF reports a sidecar
	// that cannot be read.
	if n, err := opts.CreditPages(paths); err != nil {
		log.Printf("count credits pages: %v", err)
	} else {
		pages += n
	}
	allSquare := exact && len(ratios) > 0
	for _, r := range ratios {
		allSquare = allSquare && sheet.IsSquare(r)
	}
	info := PrintPagesInfo{Pages: pages, AllSquare: allSquare, Exact: exact}
	if exact {
		info.WidePerPage = widePerPage(ratios, opts)
	}
	return info, nil
}

// widePerPage is PrintPagesInfo.WidePerPage for images of the given aspect
// ratios with the print settings opts; nil when no size is listed.
func widePerPage(ratios []float64, opts sheet.Options) map[string]int {
	f, err := sheet.FormatByID(opts.Format)
	if err != nil || !f.ShapedCells || len(ratios) == 0 {
		return nil
	}
	// Cropped photos all take the same piece.
	if c, err := sheet.CropModeByID(opts.Crop); err != nil || f.UsesCrop && c.ID != sheet.DefaultCrop {
		return nil
	}
	sorted := slices.Clone(ratios)
	slices.Sort(sorted)
	middle := sorted[len(sorted)/2]
	var out map[string]int
	for _, s := range sheet.Sizes {
		o := opts
		o.Size = s.ID
		// Every size fits every paper (sheet tests guard this), so an
		// error only leaves the size out.
		g, err := o.Grid()
		if err != nil {
			continue
		}
		if slices.ContainsFunc(ratios, func(r float64) bool { return !g.Wide(r) }) {
			continue
		}
		n, err := o.PerPageFor(middle)
		if err != nil {
			continue
		}
		if out == nil {
			out = make(map[string]int, len(sheet.Sizes))
		}
		out[s.ID] = n
	}
	return out
}

// ExportPDF asks where to save, then writes the print sheet PDF of the
// images at paths, in that order, with the print settings opts. It returns
// the saved path, or "" with no error when the dialog is cancelled.
func (a *App) ExportPDF(paths []string, defaultDir string, opts sheet.Options) (string, error) {
	if len(paths) == 0 {
		return "", errors.New(a.t("err.selectCover"))
	}
	// Reject bad settings before asking where to save.
	if _, err := opts.Grid(); err != nil {
		return "", err
	}
	name, err := pdfFileName(opts)
	if err != nil {
		return "", err
	}
	dialog := runtime.SaveDialogOptions{
		Title:                a.t("dialog.exportPDF"),
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: a.t("filter.pdf"), Pattern: "*.pdf"},
		},
	}
	if info, err := os.Stat(defaultDir); err == nil && info.IsDir() {
		dialog.DefaultDirectory = defaultDir
	}
	path, err := saveFileDialog(a.ctx, dialog)
	if err != nil || path == "" {
		return "", err
	}
	if path, err = a.withExtension(path, ".pdf"); err != nil || path == "" {
		return "", err
	}
	if err := exportTo(path, paths, a.withLabels(opts)); err != nil {
		return "", err
	}
	return path, nil
}

// PreviewInfo is a rendered preview: the URL the asset handler serves its
// PDF at, and its page count.
type PreviewInfo struct {
	URL   string `json:"url"`
	Pages int    `json:"pages"`
}

// PreviewPDF renders the print sheet PDF of the images at paths with the
// print settings opts, exactly as ExportPDF writes it, and keeps it in
// memory for the preview. It reports the same errors as ExportPDF.
func (a *App) PreviewPDF(paths []string, opts sheet.Options) (PreviewInfo, error) {
	if len(paths) == 0 {
		return PreviewInfo{}, errors.New(a.t("err.selectCover"))
	}
	opts = a.withLabels(opts)
	return a.preview(func(w io.Writer) error { return buildCovers(w, paths, opts) })
}

// PreviewPage renders the PDF of the page kind with the shared settings page
// and options (as for PagePages), exactly as ExportPage writes it, and keeps
// it in memory for the preview.
func (a *App) PreviewPage(kind string, page sheet.PageSettings, options string) (PreviewInfo, error) {
	page = a.withPageLabels(page)
	return a.preview(func(w io.Writer) error { return buildPage(w, kind, page, options) })
}

// preview renders a PDF with build into the preview store.
func (a *App) preview(build func(io.Writer) error) (PreviewInfo, error) {
	var buf bytes.Buffer
	if err := build(&buf); err != nil {
		return PreviewInfo{}, err
	}
	data := buf.Bytes()
	return PreviewInfo{URL: a.previews.Put(data), Pages: preview.PageCount(data)}, nil
}

// withLabels returns opts with the stub, card and credits page labels in
// the UI language.
func (a *App) withLabels(opts sheet.Options) sheet.Options {
	opts.Labels = sheet.Labels{
		Admit:    a.t("pdf.stubAdmit"),
		Date:     a.t("pdf.stubDate"),
		Where:    a.t("pdf.stubWhere"),
		CardDate: a.t("pdf.cardDate"),
		Credits:  a.t("pdf.credits"),
		Fonts:    a.t("pdf.fonts"),
	}
	return opts
}

// withPageLabels returns page with the labels of generated pages (month and
// weekday names, moon marks, the visited map's title and caption, the habit
// tracker's title, the quote cards' title and Wikiquote credit, the recipe
// card words, the kitchen conversion chart's words) and the language of
// country names in the UI language.
func (a *App) withPageLabels(page sheet.PageSettings) sheet.PageSettings {
	page.Labels = sheet.PageLabels{
		Months:       strings.Split(a.t("pdf.months"), ","),
		Weekdays:     strings.Split(a.t("pdf.weekdays"), ","),
		NewMoon:      a.t("pdf.moonNew"),
		FullMoon:     a.t("pdf.moonFull"),
		MapTitle:     a.t("pdf.mapTitle"),
		MapVisited:   a.t("pdf.mapVisited"),
		HabitsTitle:  a.t("pdf.habitsTitle"),
		QuotesTitle:  a.t("pdf.quotesTitle"),
		QuotesCredit: a.t("pdf.quotesCredit"),

		RecipeIngredients: a.t("pdf.recipeIngredients"),
		RecipeSteps:       a.t("pdf.recipeSteps"),
		RecipeServes:      a.t("pdf.recipeServes"),
		RecipeTime:        a.t("pdf.recipeTime"),
		RecipeContinued:   a.t("pdf.recipeContinued"),
		Conversions: sheet.ConversionLabels{
			Title:       a.t("pdf.conversionsTitle"),
			Volume:      a.t("pdf.conversionVolume"),
			Weight:      a.t("pdf.conversionWeight"),
			Oven:        a.t("pdf.conversionOven"),
			CupsToGrams: a.t("pdf.conversionCupsToGrams"),
			Cup:         a.t("pdf.conversionCup"),
			Tbsp:        a.t("pdf.conversionTbsp"),
			Tsp:         a.t("pdf.conversionTsp"),
			GasMark:     a.t("pdf.conversionGasMark"),
			FanOven:     a.t("pdf.conversionFanOven"),
			Ingredients: strings.Split(a.t("pdf.conversionIngredients"), ","),
		},
		Language: string(a.language()),
	}
	return page
}

// pdfFileName is the default name of the PDF opts describe, e.g.
// "covers_A4_Standard.pdf", "covers_A4_Standard_bleed.pdf",
// "covers_A4_instax.pdf", "covers_A4_Large_square.pdf", "cards_A4.pdf",
// "spines_A4_Mini.pdf", "stubs_A4.pdf", "fullpage_A4.pdf",
// "checklist_A4.pdf" or "polaroid_A4_Standard.pdf".
// The format, paper and, when they apply, size, a crop mode other than
// None and a cut mode other than the default tell sheets with other
// settings apart.
func pdfFileName(opts sheet.Options) (string, error) {
	format, err := sheet.FormatByID(opts.Format)
	if err != nil {
		return "", err
	}
	paper, err := sheet.PaperByID(opts.Paper)
	if err != nil {
		return "", err
	}
	crop, err := sheet.CropModeByID(opts.Crop)
	if err != nil {
		return "", err
	}
	if !format.UsesCrop {
		crop, _ = sheet.CropModeByID(sheet.DefaultCrop)
	}
	name := format.FileTag + "_" + paper.FileTag
	if format.UsesSize && crop.UsesSize {
		size, err := sheet.SizeByID(opts.Size)
		if err != nil {
			return "", err
		}
		name += "_" + size.FileTag
	}
	if crop.ID != sheet.DefaultCrop {
		name += "_" + crop.FileTag
	}
	if format.UsesCut && crop.UsesCut {
		cut, err := sheet.CutModeByID(opts.Cut)
		if err != nil {
			return "", err
		}
		if cut.ID != sheet.DefaultCut {
			name += "_" + cut.FileTag
		}
	}
	return name + ".pdf", nil
}

// withExtension adds ext to a saved path that lacks it. The save dialog
// only confirmed overwriting the name as typed, so it asks again before
// replacing an existing file with the added extension. It returns "" with no
// error when the user declines.
func (a *App) withExtension(path, ext string) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ext) {
		return path, nil
	}
	path += ext
	if _, err := os.Stat(path); err == nil {
		cancel := a.t("replace.cancel")
		answer, err := messageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:          runtime.QuestionDialog,
			Title:         a.t("replace.title"),
			Message:       a.t("replace.message", filepath.Base(path)),
			Buttons:       []string{a.t("replace.confirm"), cancel},
			DefaultButton: cancel,
			CancelButton:  cancel,
		})
		if err != nil || !isYes(answer) {
			return "", err
		}
	}
	return path, nil
}

// exportTo writes the print sheet PDF of files to path, atomically.
func exportTo(path string, files []string, opts sheet.Options) error {
	return writeFileAtomic(path, "PDF", func(w io.Writer) error {
		return buildCovers(w, files, opts)
	})
}

// buildCovers writes the print sheet PDF of files to w. Export and preview
// both render through it, so the preview shows exactly what is exported.
func buildCovers(w io.Writer, files []string, opts sheet.Options) error {
	return sheet.Build(w, files, opts)
}

// buildPage writes the PDF of a generated page to w; like buildCovers, it
// is shared by export and preview.
func buildPage(w io.Writer, kind string, page sheet.PageSettings, options string) error {
	return sheet.BuildPage(w, kind, page, json.RawMessage(options))
}

// PageKindInfo describes a page kind for the Make pages list.
type PageKindInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	UsesCut   bool   `json:"usesCut"`   // whether the shared cut mode applies
	UsesTitle bool   `json:"usesTitle"` // whether the shared title is printed
	// Defaults is the JSON object of the kind's options with their default
	// values, e.g. {"spacing":5}; the UI sends its options back in that
	// shape.
	Defaults string `json:"defaults"`
	Default  bool   `json:"default"`
}

// PageKinds lists the page kinds, the default first.
func (a *App) PageKinds() []PageKindInfo {
	out := make([]PageKindInfo, 0, len(sheet.PageKinds))
	for i, k := range sheet.PageKinds {
		defaults, err := json.Marshal(k.Defaults)
		if err != nil {
			// Options are plain structs; this would be a programming error.
			log.Printf("page kind %s: defaults: %v", k.ID, err)
			defaults = []byte("{}")
		}
		out = append(out, PageKindInfo{
			ID:        k.ID,
			Label:     k.Label,
			UsesCut:   k.UsesCut,
			UsesTitle: k.UsesTitle,
			Defaults:  string(defaults),
			Default:   i == 0,
		})
	}
	return out
}

// quoteSearchTimeout bounds one SearchQuotes call (a search and up to
// three pages).
const quoteSearchTimeout = 20 * time.Second

// SearchQuotes finds the Wikiquote page of the person query names, in the
// UI language (English or Spanish), and returns its quotes and the other
// matching pages. When no matching page has quotes, the result names the
// best page with empty quotes and its other matches (no error, so the UI
// can offer them). An empty query or a query without any page gives a
// translated error.
func (a *App) SearchQuotes(query string) (quotes.Result, error) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, quoteSearchTimeout)
	defer cancel()
	lang := "en"
	if a.language() == i18n.ES {
		lang = "es"
	}
	res, err := a.quotes.Search(ctx, query, lang)
	switch {
	case errors.Is(err, quotes.ErrEmptyQuery):
		return quotes.Result{}, errors.New(a.t("err.quoteQuery"))
	case errors.Is(err, quotes.ErrNotFound):
		// The query is user text: substituted, never a format string.
		name := strings.Join(strings.Fields(query), " ")
		return quotes.Result{}, errors.New(strings.ReplaceAll(a.t("err.noQuotes"), "{query}", name))
	case err != nil:
		return quotes.Result{}, err
	}
	return res, nil
}

// PagePages counts the pages of the page kind with the shared settings page
// and options, the kind's options as a JSON object ("" for the defaults).
// Options travel as JSON so new kinds never change the bindings.
func (a *App) PagePages(kind string, page sheet.PageSettings, options string) (int, error) {
	return sheet.PageCount(kind, page, json.RawMessage(options))
}

// ExportPage asks where to save a PDF of the page kind, with the shared
// settings page and options (as for PagePages), starting in defaultDir if
// it is a folder, and writes it. It returns the saved path, or "" with no
// error when the dialog is cancelled. Bad settings are reported before the
// dialog opens.
func (a *App) ExportPage(kind, defaultDir string, page sheet.PageSettings, options string) (string, error) {
	page = a.withPageLabels(page)
	if _, err := sheet.PageCount(kind, page, json.RawMessage(options)); err != nil {
		return "", err
	}
	name, err := pageFileName(kind, page)
	if err != nil {
		return "", err
	}
	dialog := runtime.SaveDialogOptions{
		Title:                a.t("dialog.exportPDF"),
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: a.t("filter.pdf"), Pattern: "*.pdf"},
		},
	}
	if info, err := os.Stat(defaultDir); err == nil && info.IsDir() {
		dialog.DefaultDirectory = defaultDir
	}
	path, err := saveFileDialog(a.ctx, dialog)
	if err != nil || path == "" {
		return "", err
	}
	if path, err = a.withExtension(path, ".pdf"); err != nil || path == "" {
		return "", err
	}
	err = writeFileAtomic(path, "PDF", func(w io.Writer) error {
		return buildPage(w, kind, page, options)
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// pageFileName is the default name of a generated page's PDF, like
// pdfFileName: the kind, the paper and, for kinds that use it, a cut mode
// other than the default, e.g. "dotgrid_A5.pdf".
func pageFileName(kind string, page sheet.PageSettings) (string, error) {
	k, err := sheet.PageKindByID(kind)
	if err != nil {
		return "", err
	}
	paper, err := sheet.PaperByID(page.Paper)
	if err != nil {
		return "", err
	}
	name := k.FileTag + "_" + paper.FileTag
	if k.UsesCut {
		cut, err := sheet.CutModeByID(page.Cut)
		if err != nil {
			return "", err
		}
		if cut.ID != sheet.DefaultCut {
			name += "_" + cut.FileTag
		}
	}
	return name + ".pdf", nil
}

// writeFileAtomic writes a file with write. It writes a temporary file next
// to path and renames it, so a failure never leaves a partial file behind or
// damages an existing one. kind names the file in errors, e.g. "PDF".
func writeFileAtomic(path, kind string, write func(io.Writer) error) error {
	// A short fixed pattern keeps the temporary name within the file name
	// limit even when the target's name is close to it.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bujo-helper-*.tmp")
	if err != nil {
		return fmt.Errorf("create %s: %w", kind, err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if err := write(tmp); err != nil {
		return err
	}
	// CreateTemp makes the file private (0600); a saved file gets the usual
	// permissions. Windows has no permission bits to set.
	if goruntime.GOOS != "windows" {
		if err := tmp.Chmod(0o644); err != nil {
			return fmt.Errorf("write %s: %w", kind, err)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", kind, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("save %s: %w", kind, err)
	}
	ok = true
	return nil
}

// OpenFile opens the PDF file at path with its default application. It
// only opens exported PDFs, so any other path (another kind of file, a
// folder) is an error and nothing is launched.
func (a *App) OpenFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New(a.t("err.noFileToOpen"))
	}
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		return errors.New(a.t("err.notPDF", filepath.Base(path)))
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("open file: %s is not a file", filepath.Base(path))
	}
	if err := openWithSystem(path); err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	return nil
}

// openWithSystem hands path to the system opener without waiting for it.
// It is a variable so tests can check what is opened without launching
// anything.
var openWithSystem = func(path string) error {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// explorer exits with status 1 even on success, so the exit status is
	// ignored; Wait only reaps the process.
	go cmd.Wait()
	return nil
}

// isYes reports whether a question dialog answer confirms the action,
// ignoring case. GTK returns the button label, in any UI language; Windows
// returns "Yes"/"Ok" for its default buttons.
func isYes(answer string) bool {
	for _, yes := range append([]string{"yes", "ok"}, i18n.All("replace.confirm")...) {
		if strings.EqualFold(answer, yes) {
			return true
		}
	}
	return false
}
