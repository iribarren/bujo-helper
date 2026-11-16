package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"bujo-helper/internal/isbn"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// OpenLibrary searches openlibrary.org and its covers API. Images are
// smaller than Apple's but the catalogue is free and open.
//
// When Options.Language is set it first looks for editions in that language
// and returns them before the plain search results. Those editions carry
// their ISBN, and their best image is the high-resolution cover that the
// Amazon image CDN serves for the ISBN-10.
type OpenLibrary struct {
	Client       *http.Client
	BaseURL      string
	CoverBaseURL string
	// AmazonImageBaseURL is the Amazon image CDN used for edition covers.
	AmazonImageBaseURL string
}

// NewOpenLibrary returns an Open Library provider using client (or a default one).
func NewOpenLibrary(client *http.Client) *OpenLibrary {
	if client == nil {
		client = web.NewClient()
	}
	return &OpenLibrary{
		Client:             client,
		BaseURL:            "https://openlibrary.org/search.json",
		CoverBaseURL:       "https://covers.openlibrary.org",
		AmazonImageBaseURL: amazonImageBaseURL,
	}
}

// Name implements media.Provider.
func (o *OpenLibrary) Name() string { return "openlibrary" }

type openLibraryDoc struct {
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	CoverI           int64    `json:"cover_i"`
	FirstPublishYear int      `json:"first_publish_year"`
	Editions         struct {
		Docs []struct {
			Title  string   `json:"title"`
			ISBN   []string `json:"isbn"`
			CoverI int64    `json:"cover_i"`
		} `json:"docs"`
	} `json:"editions"`
}

type openLibraryResponse struct {
	Docs []openLibraryDoc `json:"docs"`
}

// Search implements media.Provider.
func (o *OpenLibrary) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	var editions []media.Candidate
	if opts.Language != "" {
		// Best effort: on any failure the plain search below still runs.
		editions, _ = o.searchEditions(ctx, item, opts.Language)
	}

	q := url.Values{
		"title":  {strings.TrimSpace(item.Title)},
		"limit":  {"25"},
		"fields": {"title,author_name,cover_i,first_publish_year"},
	}
	if creator := strings.TrimSpace(item.Creator); creator != "" {
		q.Set("author", creator)
	}
	var body openLibraryResponse
	if err := getJSON(ctx, o.Client, o.BaseURL+"?"+q.Encode(), &body); err != nil {
		if len(editions) > 0 {
			return editions, nil
		}
		return nil, err
	}

	var hits []media.Candidate
	for _, d := range body.Docs {
		if d.CoverI == 0 {
			continue
		}
		hits = append(hits, media.Candidate{
			Title:     d.Title,
			Creator:   docCreator(d),
			Year:      docYear(d),
			ImageURLs: []string{o.coverURL(d.CoverI)},
			Source:    o.Name(),
		})
	}
	if len(editions)+len(hits) == 0 {
		return nil, fmt.Errorf("%w: no results with a cover", media.ErrNotFound)
	}
	return append(editions, rankCandidates(hits, item)...), nil
}

// searchEditions finds works matching item that have an edition in lang and
// returns one candidate per edition, ranked by title.
func (o *OpenLibrary) searchEditions(ctx context.Context, item media.Item, lang string) ([]media.Candidate, error) {
	query := "title:(" + solrTerms(item.Title) + ")"
	if creator := solrTerms(item.Creator); creator != "" {
		query += " author:(" + creator + ")"
	}
	query += " language:" + solrTerms(lang)
	q := url.Values{
		"q":      {query},
		"limit":  {"10"},
		"fields": {"key,title,author_name,first_publish_year,cover_i,editions,editions.key,editions.title,editions.isbn,editions.cover_i,editions.language"},
	}
	var body openLibraryResponse
	if err := getJSON(ctx, o.Client, o.BaseURL+"?"+q.Encode(), &body); err != nil {
		return nil, err
	}

	var hits []media.Candidate
	for _, d := range body.Docs {
		for _, e := range d.Editions.Docs {
			isbn13 := editionISBN(e.ISBN)
			var urls []string
			if isbn10 := isbn.To10(isbn13); isbn10 != "" {
				urls = append(urls, asinImageURL(o.AmazonImageBaseURL, isbn10))
			}
			if e.CoverI != 0 {
				urls = append(urls, o.coverURL(e.CoverI))
			}
			if d.CoverI != 0 {
				urls = append(urls, o.coverURL(d.CoverI))
			}
			if len(urls) == 0 {
				continue
			}
			title := e.Title
			if title == "" {
				title = d.Title
			}
			hits = append(hits, media.Candidate{
				Title:     title,
				Creator:   docCreator(d),
				Year:      docYear(d),
				ImageURLs: urls,
				Source:    o.Name(),
				ISBN:      isbn13,
			})
		}
	}
	return rankCandidates(hits, item), nil
}

// coverURL is the large Open Library cover; default=false makes a missing
// cover a 404 instead of a blank image.
func (o *OpenLibrary) coverURL(id int64) string {
	return fmt.Sprintf("%s/b/id/%d-L.jpg?default=false", o.CoverBaseURL, id)
}

// editionISBN returns the first valid ISBN of an edition as an ISBN-13,
// preferring a listed ISBN-13 over a converted ISBN-10.
func editionISBN(codes []string) string {
	var from10 string
	for _, c := range codes {
		n, ok := isbn.Normalize(c)
		switch {
		case !ok:
		case len(n) == 13:
			return n
		case from10 == "":
			from10 = isbn.To13(n)
		}
	}
	return from10
}

func docCreator(d openLibraryDoc) string {
	authors := d.AuthorName
	if len(authors) > 2 {
		authors = authors[:2]
	}
	return strings.Join(authors, ", ")
}

func docYear(d openLibraryDoc) string {
	if d.FirstPublishYear > 0 {
		return strconv.Itoa(d.FirstPublishYear)
	}
	return ""
}

// solrTerms keeps only the words of s, so user input cannot break the
// fielded query syntax: punctuation such as parentheses, colons and quotes
// is dropped and the AND/OR/NOT operators are lowercased into plain words.
func solrTerms(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' && r != '.'
	})
	for i, w := range words {
		if w == "AND" || w == "OR" || w == "NOT" {
			words[i] = strings.ToLower(w)
		}
	}
	return strings.Join(words, " ")
}
