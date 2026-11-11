// Package downloader runs a batch of cover downloads: for each item it asks
// the media type's providers in order, validates the candidate images and
// saves the first good one.
package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

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

	job := &itemJob{mt: mt, client: client, outDir: req.OutputDir, opts: req.Options, maxCandidates: max(d.MaxCandidates, 1)}
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

	var reasons []string
	for _, p := range j.mt.Providers {
		if ctx.Err() != nil {
			return cancelled()
		}
		path, matched, err := j.tryProvider(ctx, p, item)
		if err == nil {
			res.Status, res.Path, res.Source, res.Matched = StatusOK, path, p.Name(), matched
			return res
		}
		reasons = append(reasons, p.Name()+": "+err.Error())
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

// tryProvider searches one provider and saves the first valid image of its
// best candidates.
func (j *itemJob) tryProvider(ctx context.Context, p media.Provider, item media.Item) (path, matched string, err error) {
	cands, err := p.Search(ctx, item, j.opts)
	if err != nil {
		return "", "", err
	}
	if len(cands) > j.maxCandidates {
		cands = cands[:j.maxCandidates]
	}
	for _, c := range cands {
		for _, u := range c.ImageURLs {
			data, contentType, ok := fetchImage(ctx, j.client, u)
			if !ok {
				continue
			}
			path, err := saveUnique(j.outDir, fileName(item, imageExt(u, contentType)), data)
			if err != nil {
				return "", "", err
			}
			return path, describe(c), nil
		}
	}
	return "", "", errors.New("no downloadable cover image")
}

// fetchImage downloads url and reports whether it is a real image.
func fetchImage(ctx context.Context, client *http.Client, url string) ([]byte, string, bool) {
	resp, err := web.Get(ctx, client, url, web.SetBrowserHeaders)
	if err != nil || !resp.OK() {
		return nil, "", false
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") || len(resp.Body) <= minImageSize {
		return nil, "", false
	}
	return resp.Body, contentType, true
}

func describe(c media.Candidate) string {
	if c.Creator == "" {
		return c.Title
	}
	return c.Title + " - " + c.Creator
}
