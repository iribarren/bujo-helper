package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"cover-downloader/internal/batch"
	"cover-downloader/internal/catalog"
	"cover-downloader/internal/downloader"
	"cover-downloader/internal/gallery"
	"cover-downloader/internal/media"
	"cover-downloader/internal/settings"
	"cover-downloader/internal/sheet"
	"cover-downloader/internal/web"
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
	Error   string `json:"error"`
}

// DoneEvent is the payload of EventDone. Error is set when the job could
// not run at all.
type DoneEvent struct {
	OK        int    `json:"ok"`
	Failed    int    `json:"failed"`
	Cancelled int    `json:"cancelled"`
	Error     string `json:"error,omitempty"`
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

	// settingsPath is the settings file; "" means it is unknown and
	// SaveSettings fails. settings is the current copy, read live by
	// tmdbKey so a saved key is used without a restart.
	settingsMu   sync.Mutex
	settingsPath string
	settings     settings.Settings
}

// NewApp returns an App with the built-in media types and the user's
// settings.
func NewApp() *App {
	path, err := settings.Path()
	if err != nil {
		log.Printf("settings: %v", err)
	}
	return newDefaultApp(web.NewClient(), path)
}

// newDefaultApp returns an App with the built-in media types and the
// settings file at path. The movie sources read the TMDB key from the
// App's live settings.
func newDefaultApp(client *http.Client, path string) *App {
	var a *App
	// The catalog is built before the App exists; a is set before any
	// search can call the key func.
	reg := catalog.Default(client, catalog.Keys{TMDB: func() string { return a.tmdbKey() }})
	a = newApp(reg, client)
	a.loadSettings(path)
	return a
}

func newApp(reg *media.Registry, client *http.Client) *App {
	return &App{
		registry:   reg,
		downloader: downloader.Downloader{Registry: reg, Client: client},
		thumbs:     gallery.NewThumbs(),
	}
}

// loadSettings reads the settings file at path and remembers path for
// SaveSettings. A read error is logged and leaves the settings empty: the
// app still works, only without the saved keys.
func (a *App) loadSettings(path string) {
	var s settings.Settings
	if path != "" {
		var err error
		if s, err = settings.Load(path); err != nil {
			log.Printf("settings: %v", err)
		}
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	a.settingsPath = path
	a.settings = s
}

// GetSettings returns the current settings.
func (a *App) GetSettings() settings.Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings
}

// SaveSettings saves s to the settings file and uses it at once.
func (a *App) SaveSettings(s settings.Settings) error {
	s.TMDBAPIKey = strings.TrimSpace(s.TMDBAPIKey)
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if a.settingsPath == "" {
		return errors.New("save settings: the settings file location is unknown")
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
		out = append(out, MediaTypeInfo{ID: t.ID, Label: t.Label, CreatorLabel: t.CreatorLabel})
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
		Title: "Import CSV",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	return importCSVFile(path)
}

func importCSVFile(path string) (string, error) {
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
		return "", fmt.Errorf("%s: no items found", name)
	}
	return batch.FormatLines(items), nil
}

// The native dialogs, as variables so tests can replace them.
var (
	saveFileDialog = runtime.SaveFileDialog
	messageDialog  = runtime.MessageDialog
)

// SaveSampleCSV asks where to save a sample CSV for the media type with the
// given ID, which the user can use as a reference for Import CSV. It returns
// the saved path, or "" with no error when the dialog is cancelled.
func (a *App) SaveSampleCSV(mediaType string) (string, error) {
	name, data, err := a.sampleCSV(mediaType)
	if err != nil {
		return "", err
	}
	path, err := saveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Save sample CSV",
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"},
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
// media type: a "title,<creator>" header and its sample items. It starts
// with a UTF-8 BOM so Excel on Windows reads accented names correctly.
func (a *App) sampleCSV(mediaType string) (string, []byte, error) {
	t, ok := a.registry.Get(mediaType)
	if !ok {
		return "", nil, fmt.Errorf("unknown media type %q", mediaType)
	}
	var b bytes.Buffer
	b.WriteString("\xef\xbb\xbf")
	header := []string{"title", strings.ToLower(t.CreatorLabel)}
	if err := batch.WriteCSV(&b, header, t.Samples); err != nil {
		return "", nil, err
	}
	return "sample_" + t.ID + ".csv", b.Bytes(), nil
}

// ChooseFolder asks for an output folder, starting at current. It returns
// "" with no error when the dialog is cancelled.
func (a *App) ChooseFolder(current string) (string, error) {
	opts := runtime.OpenDialogOptions{Title: "Choose output folder", CanCreateDirectories: true}
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
		return nil, errors.New("no items: enter one \"Title, Creator\" per line")
	}
	if _, ok := a.registry.Get(req.MediaType); !ok {
		return nil, fmt.Errorf("unknown media type %q", req.MediaType)
	}
	outDir := strings.TrimSpace(req.OutputDir)
	if outDir == "" {
		return nil, errors.New("choose an output folder")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output folder: %w", err)
	}
	country := strings.ToLower(strings.TrimSpace(req.Country))
	if country == "" {
		country = defaultCountry
	}

	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return nil, errors.New("a download is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.mu.Unlock()

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
		})
	})
	done := DoneEvent{OK: sum.OK, Failed: sum.Failed, Cancelled: sum.Cancelled}
	if err != nil {
		done.Error = err.Error()
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
		return errors.New("no folder to open")
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

// PrintSizeInfo describes a cover size preset for the Print sheet tab.
type PrintSizeInfo struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	CoverHeight float64 `json:"coverHeight"` // mm
	PerPage     int     `json:"perPage"`     // covers on a full page
	Default     bool    `json:"default"`
}

// PrintSizes lists the cover size presets, smallest first, with how many
// covers fit on a page, so the UI can count pages without its own copy of
// the layout rules.
func (a *App) PrintSizes() []PrintSizeInfo {
	out := make([]PrintSizeInfo, 0, len(sheet.Sizes))
	for _, s := range sheet.Sizes {
		g, err := sheet.Options{Size: s.ID}.Grid()
		if err != nil {
			// Every preset fits the page; sheet tests guard this.
			log.Printf("print size %s: %v", s.ID, err)
			continue
		}
		out = append(out, PrintSizeInfo{
			ID:          s.ID,
			Label:       s.Label,
			CoverHeight: s.CoverHeight,
			PerPage:     g.PerPage(),
			Default:     s.ID == sheet.DefaultSize,
		})
	}
	return out
}

// ExportPDF asks where to save, then writes the print sheet PDF of the
// images at paths, in that order, with the print settings opts. It returns
// the saved path, or "" with no error when the dialog is cancelled.
func (a *App) ExportPDF(paths []string, defaultDir string, opts sheet.Options) (string, error) {
	if len(paths) == 0 {
		return "", errors.New("select at least one cover")
	}
	// Reject bad settings before asking where to save.
	if _, err := opts.Grid(); err != nil {
		return "", err
	}
	dialog := runtime.SaveDialogOptions{
		Title:                "Export PDF",
		DefaultFilename:      "covers.pdf",
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF files (*.pdf)", Pattern: "*.pdf"},
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
	if err := exportTo(path, paths, opts); err != nil {
		return "", err
	}
	return path, nil
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
		answer, err := messageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:          runtime.QuestionDialog,
			Title:         "Replace file?",
			Message:       filepath.Base(path) + " already exists. Replace it?",
			Buttons:       []string{"Replace", "Cancel"},
			DefaultButton: "Cancel",
			CancelButton:  "Cancel",
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
		return sheet.Build(w, files, opts)
	})
}

// writeFileAtomic writes a file with write. It writes a temporary file next
// to path and renames it, so a failure never leaves a partial file behind or
// damages an existing one. kind names the file in errors, e.g. "PDF".
func writeFileAtomic(path, kind string, write func(io.Writer) error) error {
	// A short fixed pattern keeps the temporary name within the file name
	// limit even when the target's name is close to it.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cover-downloader-*.tmp")
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

// OpenFile opens the file at path with its default application.
func (a *App) OpenFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("no file to open")
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
func openWithSystem(path string) error {
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

// isYes reports whether a question dialog answer confirms the action. GTK
// returns the button label; Windows returns "Yes"/"Ok" for its default
// buttons.
func isYes(answer string) bool {
	switch strings.ToLower(answer) {
	case "replace", "yes", "ok":
		return true
	}
	return false
}
