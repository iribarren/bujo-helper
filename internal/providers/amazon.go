package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"cover-downloader/internal/isbn"
	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// ErrBotCheck reports that Amazon served a bot check (captcha or the Akamai
// "bm-verify" interstitial) instead of search results.
var ErrBotCheck = errors.New("blocked by bot check")

// amazonDomains maps country codes to Amazon domains, for the few that are
// not just the code itself.
var amazonDomains = map[string]string{
	"us": "com", "gb": "co.uk", "uk": "co.uk", "jp": "co.jp",
	"br": "com.br", "mx": "com.mx", "au": "com.au",
}

// AmazonDomain returns the Amazon top-level domain for a country code.
func AmazonDomain(countryCode string) string {
	c := strings.ToLower(strings.TrimSpace(countryCode))
	if c == "" {
		c = "us"
	}
	if d, ok := amazonDomains[c]; ok {
		return d
	}
	return c
}

// amazonImageBaseURL is Amazon's public image CDN. Unlike the store search
// page it serves covers by ASIN (an ISBN-10 for print books) without bot checks.
const amazonImageBaseURL = "https://images-na.ssl-images-amazon.com"

// Amazon scrapes Amazon book search results. It may be blocked by bot checks,
// so it is best used as a last resort.
type Amazon struct {
	Client *http.Client
	// BaseURL overrides the store URL (e.g. for tests). When empty the store
	// is https://www.amazon.<domain> for the requested country.
	BaseURL      string
	ImageBaseURL string
	// RetryDelay is the pause before retrying a blocked request.
	RetryDelay time.Duration
	// Attempts is the total number of requests made when blocked.
	Attempts int
}

// NewAmazon returns an Amazon provider using client (or a default one).
func NewAmazon(client *http.Client) *Amazon {
	if client == nil {
		client = web.NewClient()
	}
	return &Amazon{
		Client:       client,
		ImageBaseURL: amazonImageBaseURL,
		RetryDelay:   3 * time.Second,
		Attempts:     2,
	}
}

// Name implements media.Provider.
func (a *Amazon) Name() string { return "amazon" }

var (
	// sizeModifier matches the "._AC_UY218_" segment before the extension of
	// Amazon image URLs; removing it yields the original full-size image.
	sizeModifier = regexp.MustCompile(`\._[^/]*_(\.\w+)$`)
	dpASIN       = regexp.MustCompile(`/dp/([A-Z0-9]{10})`)
	byCreator    = regexp.MustCompile(`(?:^|\|)\s*by\s+([^|]+)`)
	fourDigits   = regexp.MustCompile(`\b(\d{4})\b`)
	spaces       = regexp.MustCompile(`\s+`)
)

// Search implements media.Provider. Candidates keep Amazon's own order.
func (a *Amazon) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	base := a.BaseURL
	if base == "" {
		base = "https://www.amazon." + AmazonDomain(opts.Country)
	}
	u := base + "/s?k=" + url.QueryEscape(searchTerm(item)) + "&i=stripbooks"

	body, err := a.fetchSearchPage(ctx, u)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse search page: %w", err)
	}

	var hits []media.Candidate
	doc.Find(`div[data-component-type="s-search-result"]`).Each(func(_ int, s *goquery.Selection) {
		if c, ok := a.parseResult(s); ok {
			hits = append(hits, c)
		}
	})
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no search results", media.ErrNotFound)
	}
	return hits, nil
}

// fetchSearchPage retries once when the response is not a real search page.
func (a *Amazon) fetchSearchPage(ctx context.Context, u string) ([]byte, error) {
	attempts := max(a.Attempts, 1)
	var status int
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := web.Get(ctx, a.Client, u, web.SetBrowserHeaders)
		if err != nil {
			return nil, err
		}
		if resp.OK() && isSearchPage(resp.Body) {
			return resp.Body, nil
		}
		status = resp.Status
		if attempt < attempts {
			if err := sleep(ctx, a.RetryDelay*time.Duration(attempt)); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("%w (HTTP %d)", ErrBotCheck, status)
}

// isSearchPage reports whether html is a real search page (even with zero
// results). Bot checks lack the results container or carry "bm-verify".
func isSearchPage(html []byte) bool {
	return bytes.Contains(html, []byte("s-main-slot")) && !bytes.Contains(html, []byte("bm-verify"))
}

func (a *Amazon) parseResult(s *goquery.Selection) (media.Candidate, bool) {
	asin, _ := s.Attr("data-asin")
	img := s.Find("img.s-image").First()
	src, hasSrc := img.Attr("src")
	if asin == "" || !hasSrc {
		return media.Candidate{}, false
	}

	searchImage := fullSizeImage(src)
	c := media.Candidate{
		Title:     resultTitle(s, img),
		ImageURLs: []string{searchImage},
		Source:    a.Name(),
	}
	if row := s.Find("div.a-row.a-size-base.a-color-secondary").First(); row.Length() > 0 {
		if m := byCreator.FindStringSubmatch(row.Text()); m != nil {
			c.Creator = clean(m[1])
		}
		if m := fourDigits.FindStringSubmatch(row.Find("span.a-text-normal").Text()); m != nil {
			c.Year = m[1]
		}
	}

	type format struct{ name, asin string }
	var formats []format
	s.Find(`a.a-size-base.a-link-normal.s-underline-text[href*='/dp/']`).Each(func(_ int, l *goquery.Selection) {
		href, _ := l.Attr("href")
		if m := dpASIN.FindStringSubmatch(href); m != nil {
			formats = append(formats, format{strings.ToLower(clean(l.Text())), m[1]})
		}
	})

	audioFirst := len(formats) > 0 && isAudio(formats[0].name)

	// Print editions use their ISBN-10 as ASIN; take the first one.
	for _, f := range formats {
		if !isAudio(f.name) {
			if c.ISBN = isbn.To13(f.asin); c.ISBN != "" {
				break
			}
		}
	}
	if c.ISBN == "" && !audioFirst {
		c.ISBN = isbn.To13(asin)
	}

	// Audiobook-first results show square artwork; prefer the print/Kindle cover.
	if audioFirst {
		var urls []string
		for _, f := range formats {
			if !isAudio(f.name) {
				urls = append(urls, a.asinImage(f.asin))
			}
		}
		c.ImageURLs = append(urls, searchImage)
	}
	return c, true
}

func resultTitle(s, img *goquery.Selection) string {
	h2 := s.Find("h2").First()
	if t := clean(h2.Text()); t != "" {
		return t
	}
	if t, ok := h2.Attr("aria-label"); ok && strings.TrimSpace(t) != "" {
		return clean(t)
	}
	if t, ok := img.Attr("alt"); ok && strings.TrimSpace(t) != "" {
		return clean(t)
	}
	return "cover"
}

// asinImage asks Amazon's image service for the large (~1500px) cover.
func (a *Amazon) asinImage(asin string) string {
	return asinImageURL(a.ImageBaseURL, asin)
}

// asinImageURL is the large cover of an ASIN (an ISBN-10 for print books)
// on the Amazon image service at base.
func asinImageURL(base, asin string) string {
	return base + "/images/P/" + asin + ".01._SCRMZZZZZZ_.jpg"
}

func fullSizeImage(src string) string {
	return sizeModifier.ReplaceAllString(src, "$1")
}

func isAudio(format string) bool {
	return strings.Contains(format, "audio") || strings.Contains(format, "audible")
}

func clean(s string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
