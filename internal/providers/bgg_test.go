package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// bggRoute answers BGG API requests: exact and loose name the search
// fixtures for exact=1 and for the fallback search, thing the thing
// fixture. An empty name answers an empty search.
type bggRoute struct {
	exact, loose, thing string
}

func (r bggRoute) serve(t *testing.T) func(*http.Request) (int, []byte) {
	return func(req *http.Request) (int, []byte) {
		name := r.thing
		if strings.HasSuffix(req.URL.Path, "/search") {
			name = r.loose
			if req.URL.Query().Get("exact") == "1" {
				name = r.exact
			}
		}
		if name == "" {
			name = "search_empty.xml"
		}
		return http.StatusOK, fixture(t, "bgg/"+name)
	}
}

// bggProvider returns a BoardGameGeek provider with the token "tok" whose
// API is a routed test server.
func bggProvider(t *testing.T, route func(*http.Request) (int, []byte)) (*BoardGameGeek, *routedServer) {
	t.Helper()
	rs := &routedServer{}
	srv := rs.serve(t, route)
	p := NewBoardGameGeek(srv.Client(), func() string { return " tok " })
	p.BaseURL = srv.URL + "/xmlapi2"
	return p, rs
}

// bggCalls describes each recorded request as "<endpoint> <raw query>".
func bggCalls(rs *routedServer) []string {
	var out []string
	for _, r := range rs.requests() {
		out = append(out, strings.TrimPrefix(r.URL.Path, "/xmlapi2/")+" "+r.URL.RawQuery)
	}
	return out
}

func TestBGGNameAndLabel(t *testing.T) {
	p := NewBoardGameGeek(nil, nil)
	if p.Name() != "boardgamegeek" || p.Label() != "BoardGameGeek" {
		t.Errorf("Name, Label = %q, %q", p.Name(), p.Label())
	}
	if p.BaseURL != "https://boardgamegeek.com/xmlapi2" || p.Client == nil {
		t.Errorf("defaults = %+v", p)
	}
}

func TestBGGSearch(t *testing.T) {
	catan := bggRoute{exact: "search_catan.xml", thing: "thing_catan.xml"}
	dune := bggRoute{exact: "search_dune.xml", thing: "thing_dune.xml"}
	const (
		dune1979 = "https://cf.geekdo-images.com/dune1979__original/img/orig/pic121.jpg"
		dune2019 = "https://cf.geekdo-images.com/dune2019__original/img/orig/pic283355.jpg"
	)
	for _, tt := range []struct {
		name      string
		item      media.Item
		route     bggRoute
		wantTitle string
		wantYear  string
		wantImage string
		wantCalls []string
	}{
		{
			name:      "exact match, expansion ignored",
			item:      media.Item{Title: "Catan", Creator: "1995"},
			route:     catan,
			wantTitle: "CATAN", wantYear: "1995",
			wantImage: "https://cf.geekdo-images.com/catan__original/img/orig/pic2419375.jpg",
			wantCalls: []string{"search exact=1&query=Catan&type=boardgame", "thing id=13&stats=1"},
		},
		{
			name:      "alternate name",
			item:      media.Item{Title: "Die Siedler von Catan"},
			route:     catan,
			wantTitle: "CATAN", wantYear: "1995",
			wantImage: "https://cf.geekdo-images.com/catan__original/img/orig/pic2419375.jpg",
		},
		{
			name:      "loose search when the exact one has only expansions",
			item:      media.Item{Title: "Catan"},
			route:     bggRoute{exact: "search_expansions.xml", loose: "search_catan.xml", thing: "thing_catan.xml"},
			wantTitle: "CATAN", wantYear: "1995",
			wantImage: "https://cf.geekdo-images.com/catan__original/img/orig/pic2419375.jpg",
			wantCalls: []string{
				"search exact=1&query=Catan&type=boardgame",
				"search query=Catan&type=boardgame",
				"thing id=13&stats=1",
			},
		},
		{
			name:      "loose search when the exact one is empty",
			item:      media.Item{Title: "Catan"},
			route:     bggRoute{loose: "search_catan.xml", thing: "thing_catan.xml"},
			wantTitle: "CATAN", wantYear: "1995",
			wantImage: "https://cf.geekdo-images.com/catan__original/img/orig/pic2419375.jpg",
		},
		{
			name:      "exact year wins, from the thing when the search has none",
			item:      media.Item{Title: "Dune", Creator: "1979"},
			route:     dune,
			wantTitle: "Dune", wantYear: "1979", wantImage: dune1979,
			wantCalls: []string{
				"search exact=1&query=Dune&type=boardgame",
				"thing id=121,283355,9001,316554&stats=1",
			},
		},
		{
			name:      "year one off beats an exact year with a weaker title",
			item:      media.Item{Title: "Dune", Creator: "2020"},
			route:     dune,
			wantTitle: "Dune", wantYear: "2019", wantImage: dune2019,
		},
		{
			name:      "tie broken by number of ratings",
			item:      media.Item{Title: "Dune"},
			route:     dune,
			wantTitle: "Dune", wantYear: "2019", wantImage: dune2019,
		},
		{
			name:      "unparsable year is ignored",
			item:      media.Item{Title: "Dune", Creator: "soon"},
			route:     dune,
			wantTitle: "Dune", wantYear: "2019", wantImage: dune2019,
		},
		{
			name:      "best candidate without an image gives the next one",
			item:      media.Item{Title: "Dune", Creator: "1984"},
			route:     dune,
			wantTitle: "Dune", wantYear: "2019", wantImage: dune2019,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, rs := bggProvider(t, tt.route.serve(t))
			got, err := p.Search(context.Background(), tt.item, media.Options{})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			c := got[0]
			if c.Title != tt.wantTitle || c.Year != tt.wantYear || c.Creator != tt.wantYear || c.Source != "boardgamegeek" {
				t.Errorf("candidate = %+v, want %q (%s)", c, tt.wantTitle, tt.wantYear)
			}
			if len(c.ImageURLs) != 1 || c.ImageURLs[0] != tt.wantImage {
				t.Errorf("ImageURLs = %q, want [%q]", c.ImageURLs, tt.wantImage)
			}
			for _, o := range got {
				if len(o.ImageURLs) != 1 || o.ImageURLs[0] == "" {
					t.Errorf("candidate without an image: %+v", o)
				}
			}
			if tt.wantCalls != nil {
				if calls := bggCalls(rs); strings.Join(calls, "\n") != strings.Join(tt.wantCalls, "\n") {
					t.Errorf("requests = %q, want %q", calls, tt.wantCalls)
				}
			}
		})
	}
}

func TestBGGSendsTheBearerTokenAndContactUserAgent(t *testing.T) {
	p, rs := bggProvider(t, bggRoute{exact: "search_catan.xml", thing: "thing_catan.xml"}.serve(t))
	if _, err := p.Search(context.Background(), media.Item{Title: "Catan"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if len(reqs) != 2 {
		t.Fatalf("%d requests, want 2", len(reqs))
	}
	for _, r := range reqs {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("%s Authorization = %q, want %q", r.URL.Path, got, "Bearer tok")
		}
		if got := r.Header.Get("User-Agent"); got != web.ContactUserAgent {
			t.Errorf("%s User-Agent = %q, want the contact UA", r.URL.Path, got)
		}
	}
}

func TestBGGCredit(t *testing.T) {
	p, _ := bggProvider(t, bggRoute{exact: "search_catan.xml", thing: "thing_catan.xml"}.serve(t))
	got, err := p.Search(context.Background(), media.Item{Title: "Catan"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"Title: CATAN (1995)",
		"Image: BoardGameGeek",
		"Game page: https://boardgamegeek.com/boardgame/13",
		"Box art © its publisher; personal use only.",
	}, "\n")
	if got[0].Credit != want {
		t.Errorf("Credit =\n%s\nwant\n%s", got[0].Credit, want)
	}
}

func TestBGGCreditWithoutYear(t *testing.T) {
	got := bggCredit("Mystery Game", "", "42")
	if !strings.HasPrefix(got, "Title: Mystery Game\n") {
		t.Errorf("bggCredit() = %q, want a title line without a year", got)
	}
}

func TestBGGFetchesAtMostTwentyThings(t *testing.T) {
	var search strings.Builder
	search.WriteString(`<items total="25">`)
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&search, `<item type="boardgame" id="%d"><name type="primary" value="Game %d"/></item>`, i, i)
	}
	search.WriteString(`</items>`)
	route := func(r *http.Request) (int, []byte) {
		if strings.HasSuffix(r.URL.Path, "/search") {
			return http.StatusOK, []byte(search.String())
		}
		return http.StatusOK, []byte(`<items><item type="boardgame" id="1"><image>https://cf.geekdo-images.com/1.jpg</image>` +
			`<name type="primary" value="Game 1"/></item></items>`)
	}
	p, rs := bggProvider(t, route)
	if _, err := p.Search(context.Background(), media.Item{Title: "Game"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	ids := strings.Split(reqs[len(reqs)-1].URL.Query().Get("id"), ",")
	if len(ids) != 20 || ids[0] != "1" || ids[19] != "20" {
		t.Errorf("thing ids = %q, want the first 20", ids)
	}
}

func TestBGGNotFound(t *testing.T) {
	for _, tt := range []struct {
		name  string
		item  media.Item
		route bggRoute
		calls int
	}{
		{"no results", media.Item{Title: "Nonexistent"}, bggRoute{}, 2},
		{"only expansions", media.Item{Title: "Seafarers"}, bggRoute{exact: "search_expansions.xml", loose: "search_expansions.xml"}, 2},
		{"no image", media.Item{Title: "Catan"}, bggRoute{exact: "search_catan.xml", thing: "search_empty.xml"}, 2},
		{"empty title", media.Item{Title: "  "}, bggRoute{}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, rs := bggProvider(t, tt.route.serve(t))
			got, err := p.Search(context.Background(), tt.item, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Fatalf("Search() = %+v, %v; want ErrNotFound", got, err)
			}
			if tt.calls > 0 && !strings.Contains(err.Error(), "BoardGameGeek") {
				t.Errorf("error %q does not name BoardGameGeek", err)
			}
			if n := len(rs.requests()); n != tt.calls {
				t.Errorf("%d requests, want %d", n, tt.calls)
			}
		})
	}
}

func TestBGGWithoutTokenIsSkipped(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token func() string
	}{
		{"nil func", nil},
		{"blank token", func() string { return "  " }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, rs := bggProvider(t, bggRoute{}.serve(t))
			p.Token = tt.token
			_, err := p.Search(context.Background(), media.Item{Title: "Catan"}, media.Options{})
			if !errors.Is(err, media.ErrSkipped) || !errors.Is(err, ErrNoBGGToken) {
				t.Errorf("err = %v, want ErrNoBGGToken (skipped)", err)
			}
			if n := len(rs.requests()); n != 0 {
				t.Errorf("%d requests, want none", n)
			}
		})
	}
}

func TestBGGHTTPErrors(t *testing.T) {
	for _, tt := range []struct {
		status int
		want   string
	}{
		{http.StatusUnauthorized, "BoardGameGeek rejected the token"},
		{http.StatusForbidden, "BoardGameGeek rejected the token"},
		{http.StatusTooManyRequests, "BoardGameGeek is busy"},
		{http.StatusInternalServerError, "BoardGameGeek is busy"},
		{http.StatusServiceUnavailable, "BoardGameGeek is busy"},
		{http.StatusNotFound, "HTTP 404"},
	} {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			p, _ := bggProvider(t, func(*http.Request) (int, []byte) { return tt.status, []byte("Unauthorized") })
			p.Token = func() string { return "s3cr3t" }
			_, err := p.Search(context.Background(), media.Item{Title: "Catan"}, media.Options{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
			if errors.Is(err, media.ErrNotFound) || errors.Is(err, media.ErrSkipped) {
				t.Errorf("err = %v wraps ErrNotFound or ErrSkipped", err)
			}
			if strings.Contains(err.Error(), "s3cr3t") {
				t.Errorf("err = %v leaks the token", err)
			}
		})
	}
}

func TestBGGThingErrorIsReported(t *testing.T) {
	p, _ := bggProvider(t, func(r *http.Request) (int, []byte) {
		if strings.HasSuffix(r.URL.Path, "/thing") {
			return http.StatusServiceUnavailable, nil
		}
		return http.StatusOK, fixture(t, "bgg/search_catan.xml")
	})
	_, err := p.Search(context.Background(), media.Item{Title: "Catan"}, media.Options{})
	if err == nil || !strings.Contains(err.Error(), "BoardGameGeek is busy") {
		t.Errorf("err = %v, want BoardGameGeek is busy", err)
	}
}

func TestBGGMalformedXML(t *testing.T) {
	p, _ := bggProvider(t, func(*http.Request) (int, []byte) { return http.StatusOK, []byte("<items><item") })
	_, err := p.Search(context.Background(), media.Item{Title: "Catan"}, media.Options{})
	if err == nil || errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want a decode error", err)
	}
}
