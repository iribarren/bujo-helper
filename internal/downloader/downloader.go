// Package downloader runs a batch of cover downloads: for each item it asks
// the media type's providers in order, validates the candidate images and
// saves the first good one. When the country has known ISBN prefixes, local
// editions are tried first and any edition is the silent fallback.
package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"cover-downloader/internal/locale"
	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// Item statuses reported in ItemResult.Status.
const (
	StatusOK        = "ok"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// DefaultWorkers is the number of items processed concurrently when
// Request.Workers is not set.
const DefaultWorkers = 2

// minImageSize rejects placeholder images (Amazon and Open Library serve
// tiny GIFs for missing covers).
const minImageSize = 1024

// Request describes one batch.
type Request struct {
	MediaType string
	Items     []media.Item
	OutputDir string
	Options   media.Options
	Workers   int
}

// ItemResult reports the outcome of one item.
type ItemResult struct {
	Index   int
	Item    media.Item
	Status  string // StatusOK, StatusFailed or StatusCancelled
	Path    string // saved file, when Status is StatusOK
	Source  string // provider that supplied the image
	Matched string // "Title - Creator" of the matched candidate
	Error   string // why the item failed or was cancelled
}

// Summary counts item outcomes.
type Summary struct {
	OK, Failed, Cancelled int
}

// Downloader downloads covers using the providers in Registry. Client is
// used for image downloads; nil means a default client with a timeout.
type Downloader struct {
	Registry *media.Registry
	Client   *http.Client
	// MaxCandidates is how many of each provider's ranked candidates are
	// tried before moving on to the next provider. Zero means 1: the best
	// match only, as a lower-ranked hit is usually a different work and the
	// next provider is more likely to have the right cover.
	MaxCandidates int
}

// Run downloads a cover for every item in req and returns the counts.
//
// onResult, if not nil, is called exactly once per item as soon as that item
// finishes. It is called from worker goroutines, possibly concurrently, so it
// must be safe for concurrent use.
//
// A failing item never stops the batch. When ctx is cancelled, items in
// flight and items not yet started are reported as cancelled. The returned
// error is non-nil only when the batch cannot start (unknown media type or
// unusable output directory).
func (d *Downloader) Run(ctx context.Context, req Request, onResult func(ItemResult)) (Summary, error) {
	if d.Registry == nil {
		return Summary{}, errors.New("downloader has no registry")
	}
	mt, ok := d.Registry.Get(req.MediaType)
	if !ok {
		return Summary{}, fmt.Errorf("unknown media type %q", req.MediaType)
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		return Summary{}, errors.New("output directory is required")
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return Summary{}, fmt.Errorf("create output directory: %w", err)
	}

	client := d.Client
	if client == nil {
		client = web.NewClient()
	}
	workers := req.Workers
	if workers <= 0 {
		workers = DefaultWorkers
	}
	workers = min(workers, max(len(req.Items), 1))

	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for i := range req.Items {
			jobs <- i
		}
	}()

	var (
		mu      sync.Mutex
		summary Summary
		wg      sync.WaitGroup
	)
	report := func(r ItemResult) {
		mu.Lock()
		switch r.Status {
		case StatusOK:
			summary.OK++
		case StatusCancelled:
			summary.Cancelled++
		default:
			summary.Failed++
		}
		mu.Unlock()
		if onResult != nil {
			onResult(r)
		}
	}

	loc := locale.For(req.Options.Country)
	opts := req.Options
	if opts.Language == "" {
		opts.Language = loc.Language
	}
	job := &itemJob{mt: mt, client: client, outDir: req.OutputDir, opts: opts, loc: loc, maxCandidates: max(d.MaxCandidates, 1)}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				report(job.run(ctx, i, req.Items[i]))
			}
		}()
	}
	wg.Wait()
	return summary, nil
}

// itemJob holds what every item of a batch shares.
type itemJob struct {
	mt            media.MediaType
	client        *http.Client
	outDir        string
	opts          media.Options
	loc           locale.Locale
	maxCandidates int
}

func (j *itemJob) run(ctx context.Context, index int, item media.Item) ItemResult {
	res := ItemResult{Index: index, Item: item}
	cancelled := func() ItemResult {
		res.Status, res.Error = StatusCancelled, "cancelled"
		return res
	}
	if ctx.Err() != nil {
		return cancelled()
	}
	if strings.TrimSpace(item.Title) == "" {
		res.Status, res.Error = StatusFailed, "empty title"
		return res
	}

	// Pass 1 tries only local editions (by ISBN prefix); pass 2 tries every
	// candidate in the providers' own order. Each provider is searched once.
	s := &itemSearch{job: j, item: item, results: make([]searchResult, len(j.mt.Providers)), failed: map[string]error{}}
	var passes []func(media.Candidate) bool
	if len(j.loc.ISBNPrefixes) > 0 {
		passes = append(passes, func(c media.Candidate) bool { return j.loc.IsLocal(c.ISBN) })
	}
	passes = append(passes, nil) // nil keeps every candidate
	var reasons []string
	for pass, keep := range passes {
		last := pass == len(passes)-1
		for i, p := range j.mt.Providers {
			if ctx.Err() != nil {
				return cancelled()
			}
			path, matched, err := s.tryProvider(ctx, i, keep)
			if err == nil {
				res.Status, res.Path, res.Source, res.Matched = StatusOK, path, p.Name(), matched
				return res
			}
			if last {
				reasons = append(reasons, p.Name()+": "+err.Error())
			}
		}
	}
	if ctx.Err() != nil {
		return cancelled()
	}
	res.Status = StatusFailed
	if len(reasons) == 0 {
		res.Error = "no providers for media type " + j.mt.ID
	} else {
		res.Error = strings.Join(reasons, "; ")
	}
	return res
}

// searchResult caches one provider's answer for an item.
type searchResult struct {
	done  bool
	cands []media.Candidate
	err   error
}

// itemSearch holds the per-item state shared by both passes: each
// provider's cached search and the image URLs that already failed.
type itemSearch struct {
	job     *itemJob
	item    media.Item
	results []searchResult
	failed  map[string]error
}

// search returns provider i's candidates, asking the provider only once.
func (s *itemSearch) search(ctx context.Context, i int) ([]media.Candidate, error) {
	r := &s.results[i]
	if !r.done {
		r.cands, r.err = s.job.mt.Providers[i].Search(ctx, s.item, s.job.opts)
		r.done = true
	}
	return r.cands, r.err
}

// tryProvider saves the first valid image of provider i's best candidates.
// When keep is not nil, only the candidates it accepts are considered, and
// they are filtered before the MaxCandidates cut.
func (s *itemSearch) tryProvider(ctx context.Context, i int, keep func(media.Candidate) bool) (path, matched string, err error) {
	cands, err := s.search(ctx, i)
	if err != nil {
		return "", "", err
	}
	if keep != nil {
		var kept []media.Candidate
		for _, c := range cands {
			if keep(c) {
				kept = append(kept, c)
			}
		}
		if len(kept) == 0 {
			return "", "", errors.New("no local edition")
		}
		cands = kept
	}
	if len(cands) > s.job.maxCandidates {
		cands = cands[:s.job.maxCandidates]
	}
	var last error
	for _, c := range cands {
		for _, u := range c.ImageURLs {
			data, contentType, err := s.fetch(ctx, u)
			if err != nil {
				last = err
				continue
			}
			path, err := saveUnique(s.job.outDir, fileName(s.item, imageExt(u, contentType)), data)
			if err != nil {
				return "", "", err
			}
			return path, describe(c), nil
		}
	}
	if last != nil {
		return "", "", fmt.Errorf("no downloadable cover image (last: %w)", last)
	}
	return "", "", errors.New("no downloadable cover image")
}

// fetch downloads an image URL, remembering failures so the second pass
// does not request a rejected URL again.
func (s *itemSearch) fetch(ctx context.Context, u string) ([]byte, string, error) {
	if err, ok := s.failed[u]; ok {
		return nil, "", err
	}
	data, contentType, err := fetchImage(ctx, s.job.client, u)
	if err != nil {
		s.failed[u] = err
	}
	return data, contentType, err
}

// fetchImage downloads url and returns its body and content type when it is
// a real image. Otherwise the error says why: a transport failure (including
// web.ErrBodyTooLarge), an HTTP status, a non-image content type or an image
// small enough to be a placeholder.
func fetchImage(ctx context.Context, client *http.Client, url string) ([]byte, string, error) {
	resp, err := web.Get(ctx, client, url, web.SetBrowserHeaders)
	if err != nil {
		return nil, "", err
	}
	if !resp.OK() {
		return nil, "", fmt.Errorf("HTTP %d", resp.Status)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		if contentType == "" {
			contentType = "no content type"
		}
		return nil, "", fmt.Errorf("not an image (%s)", contentType)
	}
	if len(resp.Body) <= minImageSize {
		return nil, "", fmt.Errorf("image too small (%d bytes), likely a placeholder", len(resp.Body))
	}
	return resp.Body, contentType, nil
}

func describe(c media.Candidate) string {
	if c.Creator == "" {
		return c.Title
	}
	return c.Title + " - " + c.Creator
}
