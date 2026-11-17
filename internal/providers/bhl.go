package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// ErrNoBHLKey reports that no Biodiversity Heritage Library API key is
// configured. It does not wrap media.ErrNotFound: the provider is skipped,
// not searched.
var ErrNoBHLKey = errors.New("no BHL API key (set it in Settings)")

// errBHLKeyRejected is returned when BHL refuses the key.
var errBHLKeyRejected = errors.New("BHL rejected the API key (HTTP 401)")

// BHL finds a vintage natural-history illustration of a plant or animal in
// the Biodiversity Heritage Library (API v3). It needs a free per-user API
// key. The name is resolved to a Wikidata taxon (shared with the Commons
// provider), then GetNameMetadata lists the pages that name its taxon name
// (P225). Pages typed "Illustration" or numbered as a "Plate" are kept,
// grouped by book volume (item), oldest publication first. Each volume's
// rights come from GetItemMetadata: only public-domain, not-in-copyright
// and no-known-restrictions volumes without an NC or ND licence are used.
// The image is the page scan (/pageimage/<PageID>), which BHL serves as
// WebP; the downloader saves it as JPEG.
type BHL struct {
	Client *http.Client
	// APIURL is the API v3 endpoint.
	APIURL string
	// Key returns the current API key. Nil or blank skips the provider.
	Key func() string
	// Taxa resolves names; providers may share one to look each name up
	// once.
	Taxa *TaxonResolver
	// MaxResponseBytes caps an API response: the name metadata of a common
	// species can list thousands of pages.
	MaxResponseBytes int64
}

// NewBHL returns a BHL provider using client (or a default one), the key
// source and taxa (or a new resolver).
func NewBHL(client *http.Client, key func() string, taxa *TaxonResolver) *BHL {
	if client == nil {
		client = web.NewClient()
	}
	if taxa == nil {
		taxa = NewTaxonResolver(client)
	}
	return &BHL{
		Client:           client,
		APIURL:           "https://www.biodiversitylibrary.org/api3",
		Key:              key,
		Taxa:             taxa,
		MaxResponseBytes: 16 << 20,
	}
}

// Name implements media.Provider.
func (p *BHL) Name() string { return "bhl" }

const (
	// bhlMaxItems caps the volumes whose rights are looked up.
	bhlMaxItems = 3
	// bhlPageImageURL is the page scan URL; the page ID follows.
	bhlPageImageURL = "https://www.biodiversitylibrary.org/pageimage/"
)

// bhlID is a BHL identifier, which the API sends as a number or a string.
type bhlID string

func (id *bhlID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*id = bhlID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*id = bhlID(n.String())
	return nil
}

// bhlName is a GetNameMetadata result.
type bhlName struct {
	NameCanonical string `json:"NameCanonical"`
	Titles        []struct {
		TitleID         bhlID  `json:"TitleID"`
		ShortTitle      string `json:"ShortTitle"`
		PublicationDate string `json:"PublicationDate"`
		Items           []struct {
			ItemID             bhlID     `json:"ItemID"`
			Volume             string    `json:"Volume"`
			HoldingInstitution string    `json:"HoldingInstitution"`
			ItemURL            string    `json:"ItemUrl"`
			Pages              []bhlPage `json:"Pages"`
		} `json:"Items"`
	} `json:"Titles"`
}

type bhlPage struct {
	PageID           bhlID  `json:"PageID"`
	PageURL          string `json:"PageUrl"`
	FullSizeImageURL string `json:"FullSizeImageUrl"`
	PageTypes        []struct {
		PageTypeName string `json:"PageTypeName"`
	} `json:"PageTypes"`
	PageNumbers []struct {
		Prefix string `json:"Prefix"`
		Number string `json:"Number"`
	} `json:"PageNumbers"`
}

// bhlItem is a GetItemMetadata result: a volume's rights.
type bhlItem struct {
	ItemID             bhlID  `json:"ItemID"`
	HoldingInstitution string `json:"HoldingInstitution"`
	CopyrightStatus    string `json:"CopyrightStatus"`
	Rights             string `json:"Rights"`
	LicenseURL         string `json:"LicenseUrl"`
	ItemURL            string `json:"ItemUrl"`
}

// bhlVolume is a volume (item) with plate pages, from the name metadata.
type bhlVolume struct {
	id, title, date, volume, institution, itemURL string
	year                                          int
	plates                                        []bhlPage
}

// Search implements media.Provider. item.Title is the name; item.Creator
// is not used.
func (p *BHL) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	key := ""
	if p.Key != nil {
		key = strings.TrimSpace(p.Key())
	}
	if key == "" {
		return nil, ErrNoBHLKey
	}
	taxon, err := p.Taxa.Resolve(ctx, item.Title)
	if err != nil {
		return nil, err
	}
	if !taxon.HasTaxonName {
		return nil, fmt.Errorf("%w: %s (%s) has no taxon name (P225) to search BHL for", media.ErrNotFound, taxon.Label, taxon.ID)
	}

	var names []bhlName
	if err := p.call(ctx, key, url.Values{"op": {"GetNameMetadata"}, "name": {taxon.Name}}, &names); err != nil {
		return nil, err
	}
	volumes, pages := bhlVolumes(names)
	if len(volumes) == 0 {
		return nil, fmt.Errorf("%w: BHL has no illustration or plate naming %s (%d pages)", media.ErrNotFound, taxon.Name, pages)
	}

	var skipped []string
	for i, v := range volumes {
		if i == bhlMaxItems {
			skipped = append(skipped, fmt.Sprintf("and %d more volumes not checked", len(volumes)-i))
			break
		}
		var items []bhlItem
		if err := p.call(ctx, key, url.Values{"op": {"GetItemMetadata"}, "id": {v.id}, "idtype": {"bhl"}}, &items); err != nil {
			return nil, err
		}
		if len(items) == 0 {
			skipped = append(skipped, fmt.Sprintf("%s (item %s): no item metadata", v.title, v.id))
			continue
		}
		rights := items[0]
		if reason := bhlRights(rights.CopyrightStatus, rights.LicenseURL); reason != "" {
			skipped = append(skipped, fmt.Sprintf("%s (item %s): %s", v.title, v.id, reason))
			continue
		}
		var out []media.Candidate
		for _, page := range v.plates {
			if len(out) == natureMaxCandidates {
				break
			}
			out = append(out, media.Candidate{
				Title:     taxon.Label,
				Creator:   taxonCreator(taxon),
				ImageURLs: []string{bhlImageURL(page)},
				Source:    p.Name(),
				Credit:    bhlCredit(v, rights, page, taxon),
			})
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: no public-domain BHL illustration of %s; skipped %s", media.ErrNotFound, taxon.Name, strings.Join(skipped, "; "))
}

// bhlVolumes returns the volumes that hold illustration or plate pages,
// oldest publication first (unknown dates last; ties keep the API order),
// and how many pages the names list in all.
func bhlVolumes(names []bhlName) ([]bhlVolume, int) {
	var out []bhlVolume
	pages := 0
	seen := map[bhlID]bool{}
	for _, n := range names {
		for _, t := range n.Titles {
			for _, it := range t.Items {
				pages += len(it.Pages)
				if seen[it.ItemID] {
					continue
				}
				v := bhlVolume{
					id:          string(it.ItemID),
					title:       strings.TrimSpace(t.ShortTitle),
					date:        strings.TrimSpace(t.PublicationDate),
					volume:      strings.TrimSpace(it.Volume),
					institution: strings.TrimSpace(it.HoldingInstitution),
					itemURL:     it.ItemURL,
					year:        firstYear(t.PublicationDate + " " + it.Volume),
				}
				for _, pg := range it.Pages {
					if isBHLPlate(pg) && pg.PageID != "" {
						v.plates = append(v.plates, pg)
					}
				}
				if len(v.plates) > 0 && v.id != "" {
					seen[it.ItemID] = true
					out = append(out, v)
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].year < out[j].year })
	return out, pages
}

// isBHLPlate reports whether a page is an illustration or a numbered plate.
func isBHLPlate(pg bhlPage) bool {
	for _, t := range pg.PageTypes {
		if strings.EqualFold(strings.TrimSpace(t.PageTypeName), "Illustration") {
			return true
		}
	}
	for _, n := range pg.PageNumbers {
		if strings.EqualFold(strings.TrimSpace(n.Prefix), "Plate") {
			return true
		}
	}
	return false
}

// plateNumber returns "Plate 4" for a page numbered as a plate, else "".
func plateNumber(pg bhlPage) string {
	for _, n := range pg.PageNumbers {
		if strings.EqualFold(strings.TrimSpace(n.Prefix), "Plate") {
			return strings.TrimSpace("Plate " + strings.TrimSpace(n.Number))
		}
	}
	return ""
}

var yearPattern = regexp.MustCompile(`\b(1[5-9]\d\d|20\d\d)\b`)

// firstYear returns the first plausible year in s, or a large number when
// there is none, so undated volumes sort last.
func firstYear(s string) int {
	if m := yearPattern.FindString(s); m != "" {
		y, _ := strconv.Atoi(m)
		return y
	}
	return 1 << 30
}

// bhlImageURL is the scan of a page.
func bhlImageURL(pg bhlPage) string {
	if u := strings.TrimSpace(pg.FullSizeImageURL); u != "" {
		return u
	}
	return bhlPageImageURL + string(pg.PageID)
}

// bhlRights returns "" when a volume's copyright status and licence allow
// printing its pages (public domain, not in copyright, not specified or no
// known restrictions, with no NC or ND licence), else why it is skipped.
func bhlRights(status, licenseURL string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	u := strings.ToLower(licenseURL)
	switch {
	case strings.HasPrefix(s, "in copyright"):
		return "in copyright (" + status + ")"
	case strings.Contains(u, "-nc"):
		return "non-commercial licence (" + licenseURL + ")"
	case strings.Contains(u, "-nd"):
		return "no-derivatives licence (" + licenseURL + ")"
	case s == "", s == "not_in_copyright", strings.HasPrefix(s, "not in copyright"),
		strings.HasPrefix(s, "not specified"), strings.HasPrefix(s, "public domain"),
		strings.HasPrefix(s, "no known copyright restrictions"):
		return ""
	}
	return "not public domain (" + status + ")"
}

// bhlCredit builds the credit text of a BHL page: the line BHL asks for,
// then title, date, page, copyright status, licence, rights, volume and the
// Wikidata taxon.
func bhlCredit(v bhlVolume, rights bhlItem, pg bhlPage, taxon Taxon) string {
	institution := strings.TrimSpace(rights.HoldingInstitution)
	if institution == "" {
		institution = v.institution
	}
	first := "Image from the Biodiversity Heritage Library."
	if institution != "" {
		first += " Contributed by " + institution + "."
	}
	lines := []string{first + " | www.biodiversitylibrary.org"}
	if title := v.title; title != "" {
		if v.volume != "" {
			title += ", " + v.volume
		}
		lines = append(lines, "Title: "+title)
	}
	if v.date != "" {
		lines = append(lines, "Date: "+v.date)
	}
	page := strings.TrimSpace(pg.PageURL)
	if page == "" {
		page = "https://www.biodiversitylibrary.org/page/" + string(pg.PageID)
	}
	if plate := plateNumber(pg); plate != "" {
		page += " (" + plate + ")"
	}
	lines = append(lines, "Page: "+page)
	status := strings.TrimSpace(rights.CopyrightStatus)
	if status == "" {
		status = "Not specified"
	}
	lines = append(lines, "Copyright status: "+status)
	if l := strings.TrimSpace(rights.LicenseURL); l != "" {
		lines = append(lines, "License: "+l)
	}
	if r := strings.TrimSpace(rights.Rights); r != "" {
		lines = append(lines, "Rights: "+r)
	}
	itemURL := strings.TrimSpace(rights.ItemURL)
	if itemURL == "" {
		itemURL = v.itemURL
	}
	if itemURL != "" {
		lines = append(lines, "Item: "+itemURL)
	}
	lines = append(lines, "Wikidata taxon: https://www.wikidata.org/wiki/"+taxon.ID)
	return strings.Join(lines, "\n")
}

// bhlResponse is the API envelope. Result is a list on success and a
// message on some errors.
type bhlResponse struct {
	Status       string          `json:"Status"`
	ErrorMessage string          `json:"ErrorMessage"`
	Result       json.RawMessage `json:"Result"`
}

// call runs one API operation with key and decodes its Result into v. The
// key travels in the URL, so it is redacted from every error.
func (p *BHL) call(ctx context.Context, key string, q url.Values, v any) error {
	op := q.Get("op")
	q.Set("format", "json")
	q.Set("apikey", key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.APIURL+"?"+q.Encode(), nil)
	if err != nil {
		return redactKey(err, key)
	}
	web.SetBHLHeaders(req)
	resp, err := p.Client.Do(req)
	if err != nil {
		return redactKey(err, key)
	}
	defer resp.Body.Close()
	limit := p.MaxResponseBytes
	if limit <= 0 {
		limit = web.MaxBodySize
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return redactKey(fmt.Errorf("read BHL %s response: %w", op, err), key)
	}
	if int64(len(body)) > limit {
		return fmt.Errorf("BHL %s response: %w (over %d bytes)", op, web.ErrBodyTooLarge, limit)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errBHLKeyRejected
	}
	var env bhlResponse
	if err := json.Unmarshal(body, &env); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("BHL %s: HTTP %d", op, resp.StatusCode)
		}
		return fmt.Errorf("decode BHL %s response: %w", op, err)
	}
	if strings.EqualFold(env.Status, "unauthorized") {
		return errBHLKeyRejected
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("BHL %s: HTTP %d", op, resp.StatusCode)
	}
	if !strings.EqualFold(env.Status, "ok") {
		msg := strings.TrimSpace(env.ErrorMessage)
		if msg == "" {
			msg = "status " + strconv.Quote(env.Status)
		}
		return errors.New("BHL API error: " + redactText(msg, key))
	}
	if len(bytes.TrimSpace(env.Result)) == 0 || bytes.Equal(bytes.TrimSpace(env.Result), []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(env.Result, v); err != nil {
		return fmt.Errorf("decode BHL %s response: %w", op, err)
	}
	return nil
}

// redactText replaces key, plain or URL-escaped, in s with "REDACTED".
func redactText(s, key string) string {
	if key == "" {
		return s
	}
	s = strings.ReplaceAll(s, key, "REDACTED")
	return strings.ReplaceAll(s, url.QueryEscape(key), "REDACTED")
}
