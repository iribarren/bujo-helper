package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// routedServer answers each request with what route returns and records
// the request URLs in order.
type routedServer struct {
	mu   sync.Mutex
	reqs []*http.Request
}

func (s *routedServer) serve(t *testing.T, route func(r *http.Request) (status int, body []byte)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.reqs = append(s.reqs, r)
		s.mu.Unlock()
		status, body := route(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (s *routedServer) requests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*http.Request(nil), s.reqs...)
}

// paths returns the path of every recorded request, in order.
func (s *routedServer) paths() []string {
	var out []string
	for _, r := range s.requests() {
		out = append(out, r.URL.Path)
	}
	return out
}

const (
	metSearchPath = "/public/collection/v1.1/search"
	metObjectPath = "/public/collection/v1/objects/"
)

// fakeMet serves Met searches from searches, keyed by the q parameter plus
// " [title]" when title=true is set, and objects from the
// met_object_<id>.json fixtures listed in objects. Anything else is 404.
func fakeMet(t *testing.T, searches map[string]string, objects ...int) (*Met, *routedServer) {
	t.Helper()
	known := map[string][]byte{}
	for _, id := range objects {
		known[fmt.Sprint(id)] = fixture(t, fmt.Sprintf("met_object_%d.json", id))
	}
	var rs routedServer
	srv := rs.serve(t, func(r *http.Request) (int, []byte) {
		switch {
		case r.URL.Path == metSearchPath:
			key := r.URL.Query().Get("q")
			if r.URL.Query().Get("title") == "true" {
				key += " [title]"
			}
			if body, ok := searches[key]; ok {
				return http.StatusOK, []byte(body)
			}
			return http.StatusOK, []byte(`{"total":0,"objectIDs":null}`)
		case strings.HasPrefix(r.URL.Path, metObjectPath):
			if body, ok := known[strings.TrimPrefix(r.URL.Path, metObjectPath)]; ok {
				return http.StatusOK, body
			}
		}
		return http.StatusNotFound, []byte(`{"message":"ObjectID not found"}`)
	})
	p := NewMet(srv.Client())
	p.BaseURL = srv.URL + "/public/collection"
	return p, &rs
}

func objectPaths(ids ...int) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%s%d", metObjectPath, id)
	}
	return out
}

func TestMetSearch(t *testing.T) {
	p, rs := fakeMet(t, map[string]string{
		"Wheat Field with Cypresses [title]": `{"total":1,"objectIDs":[436535]}`,
	}, 436535)
	if p.Name() != "met" {
		t.Errorf("Name = %q, want met", p.Name())
	}

	got, err := p.Search(context.Background(), media.Item{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}

	reqs := rs.requests()
	if want := append([]string{metSearchPath}, objectPaths(436535)...); !reflect.DeepEqual(rs.paths(), want) {
		t.Fatalf("paths = %q, want %q", rs.paths(), want)
	}
	q := reqs[0].URL.Query()
	for key, want := range map[string]string{"q": "Wheat Field with Cypresses", "title": "true", "hasImages": "true"} {
		if q.Get(key) != want {
			t.Errorf("search %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if ua := reqs[0].Header.Get("User-Agent"); ua != web.APIUserAgent {
		t.Errorf("User-Agent = %q, want %q", ua, web.APIUserAgent)
	}

	want := []media.Candidate{{
		Title:   "Wheat Field with Cypresses",
		Creator: "Vincent van Gogh",
		Year:    "1889",
		// The original first, then the web-large one; never the
		// additional images.
		ImageURLs: []string{
			"https://images.metmuseum.org/CRDImages/ep/original/DP-42549-001.jpg",
			"https://images.metmuseum.org/CRDImages/ep/web-large/DP-42549-001.jpg",
		},
		Source: "met",
		Credit: "Title: Wheat Field with Cypresses (1889)\n" +
			"Artist: Vincent van Gogh\n" +
			"Museum: The Metropolitan Museum of Art\n" +
			"License: CC0 1.0 Universal (public domain), https://creativecommons.org/publicdomain/zero/1.0/\n" +
			"Object page: https://www.metmuseum.org/art/collection/search/436535",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates =\n%+v\nwant\n%+v", got, want)
	}
}

func TestMetSearchFallsBackToTitleAndArtist(t *testing.T) {
	p, rs := fakeMet(t, map[string]string{
		// The title-only search finds nothing; the free-text one does.
		"Wheat Field with Cypresses Vincent van Gogh": `{"total":2,"objectIDs":[436535,404]}`,
	}, 436535)

	got, err := p.Search(context.Background(), media.Item{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if want := append([]string{metSearchPath, metSearchPath}, objectPaths(436535, 404)...); !reflect.DeepEqual(rs.paths(), want) {
		t.Fatalf("paths = %q, want %q", rs.paths(), want)
	}
	second := reqs[1].URL.Query()
	if second.Get("q") != "Wheat Field with Cypresses Vincent van Gogh" || second.Has("title") || second.Get("hasImages") != "true" {
		t.Errorf("fallback search query = %v, want q=title artist, hasImages, no title flag", second)
	}
	// A missing object (404) is skipped.
	if len(got) != 1 || got[0].Title != "Wheat Field with Cypresses" {
		t.Errorf("candidates = %+v, want the one found object", got)
	}
}

func TestMetSearchUsesTheArtistForCommonTitles(t *testing.T) {
	ids := make([]string, metMaxObjects+2)
	for i := range ids {
		ids[i] = fmt.Sprint(100 + i)
	}
	p, rs := fakeMet(t, map[string]string{
		// Too many title matches, in no useful order: the free-text search
		// with the artist is relevance-ranked.
		"Self-Portrait [title]":   `{"total":12,"objectIDs":[` + strings.Join(ids, ",") + `]}`,
		"Self-Portrait Rembrandt": `{"total":2,"objectIDs":[742412,437397]}`,
	}, 742412, 437397)

	got, err := p.Search(context.Background(), media.Item{Title: "Self-Portrait", Creator: "Rembrandt"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := append([]string{metSearchPath, metSearchPath}, objectPaths(742412, 437397)...); !reflect.DeepEqual(rs.paths(), want) {
		t.Fatalf("paths = %q, want %q", rs.paths(), want)
	}
	// "Rembrandt Self-Portrait" by another artist is not a plausible match.
	if len(got) != 1 || got[0].Title != "Self-Portrait" || got[0].Creator != "Rembrandt (Rembrandt van Rijn)" {
		t.Errorf("candidates = %+v, want only Rembrandt's Self-Portrait", got)
	}
}

func TestMetSearchCapsObjectLookups(t *testing.T) {
	ids := make([]string, 15)
	want := []string{metSearchPath}
	for i := range ids {
		ids[i] = fmt.Sprint(100 + i)
		if i < metMaxObjects {
			want = append(want, objectPaths(100+i)...)
		}
	}
	p, rs := fakeMet(t, map[string]string{
		"Untitled [title]": `{"total":15,"objectIDs":[` + strings.Join(ids, ",") + `]}`,
	})

	_, err := p.Search(context.Background(), media.Item{Title: "Untitled"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if !reflect.DeepEqual(rs.paths(), want) {
		t.Errorf("paths = %q, want one search and %d object lookups", rs.paths(), metMaxObjects)
	}
}

func TestMetSearchSkipsWorksNotInThePublicDomain(t *testing.T) {
	p, _ := fakeMet(t, map[string]string{
		"Autumn Rhythm [title]": `{"total":1,"objectIDs":[488978]}`,
	}, 488978)

	got, err := p.Search(context.Background(), media.Item{Title: "Autumn Rhythm", Creator: "Jackson Pollock"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("err = %v (candidates %+v), want ErrNotFound", err, got)
	}
	if msg := err.Error(); !strings.Contains(msg, "1 match") || !strings.Contains(msg, "public-domain (CC0) image") {
		t.Errorf("err = %q, want it to say the match has no public-domain image", msg)
	}
}

func TestMetSearchNotFound(t *testing.T) {
	for name, tt := range map[string]struct {
		searches map[string]string
		objects  []int
		wantMsg  string
	}{
		"no search results": {wantMsg: "no artworks with images"},
		"no matching title": {
			searches: map[string]string{"Sunflowers [title]": `{"total":1,"objectIDs":[436535]}`},
			objects:  []int{436535},
			wantMsg:  "no matching artwork",
		},
	} {
		t.Run(name, func(t *testing.T) {
			p, _ := fakeMet(t, tt.searches, tt.objects...)
			_, err := p.Search(context.Background(), media.Item{Title: "Sunflowers"}, media.Options{})
			if !errors.Is(err, media.ErrNotFound) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err = %v, want ErrNotFound saying %q", err, tt.wantMsg)
			}
		})
	}
}

func TestMetSearchHTTPError(t *testing.T) {
	var rs routedServer
	srv := rs.serve(t, func(*http.Request) (int, []byte) { return http.StatusServiceUnavailable, nil })
	p := NewMet(srv.Client())
	p.BaseURL = srv.URL

	_, err := p.Search(context.Background(), media.Item{Title: "Wheat Field with Cypresses"}, media.Options{})
	if err == nil || errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want an HTTP error that is not ErrNotFound", err)
	}
}

// fakeCleveland answers the first artworks request with bodies[0], the
// second with bodies[1], and so on (the last one repeats).
func fakeCleveland(t *testing.T, bodies ...string) (*Cleveland, *routedServer) {
	t.Helper()
	var rs routedServer
	srv := rs.serve(t, func(r *http.Request) (int, []byte) {
		if r.URL.Path != "/api/artworks/" {
			return http.StatusNotFound, nil
		}
		n := min(len(rs.requests()), len(bodies)) - 1
		return http.StatusOK, fixture(t, bodies[n])
	})
	p := NewCleveland(srv.Client())
	p.BaseURL = srv.URL + "/api/artworks/"
	return p, &rs
}

// hasFlag reports whether the raw query has name as a bare flag (no "=").
func hasFlag(rawQuery, name string) bool {
	return strings.Contains("&"+rawQuery+"&", "&"+name+"&")
}

func TestClevelandSearch(t *testing.T) {
	p, rs := fakeCleveland(t, "cleveland_twilight.json")
	if p.Name() != "cleveland" {
		t.Errorf("Name = %q, want cleveland", p.Name())
	}

	got, err := p.Search(context.Background(), media.Item{Title: "Twilight in the Wilderness", Creator: "Frederic Edwin Church"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}

	reqs := rs.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	q := reqs[0].URL.Query()
	for key, want := range map[string]string{
		"title":     "Twilight in the Wilderness",
		"artists":   "Frederic Edwin Church",
		"has_image": "1",
		"limit":     "20",
		"fields":    "id,title,creators,creation_date,share_license_status,images,url",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if !hasFlag(reqs[0].URL.RawQuery, "cc0") {
		t.Errorf("raw query %q, want the bare cc0 flag", reqs[0].URL.RawQuery)
	}
	if ua := reqs[0].Header.Get("User-Agent"); ua != web.APIUserAgent {
		t.Errorf("User-Agent = %q, want %q", ua, web.APIUserAgent)
	}

	want := []media.Candidate{{
		Title:   "Twilight in the Wilderness",
		Creator: "Frederic Edwin Church",
		Year:    "1860",
		// The print image, then the web one; never the TIFF.
		ImageURLs: []string{
			"https://openaccess-cdn.clevelandart.org/1965.233/1965.233_print.jpg",
			"https://openaccess-cdn.clevelandart.org/1965.233/1965.233_web.jpg",
		},
		Source: "cleveland",
		Credit: "Title: Twilight in the Wilderness (1860)\n" +
			"Artist: Frederic Edwin Church (American, 1826–1900)\n" +
			"Museum: The Cleveland Museum of Art\n" +
			"License: CC0 1.0 Universal (public domain), https://creativecommons.org/publicdomain/zero/1.0/\n" +
			"Object page: https://clevelandart.org/art/1965.233",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates =\n%+v\nwant\n%+v", got, want)
	}
}

func TestClevelandSearchRetriesWithoutTheArtist(t *testing.T) {
	p, rs := fakeCleveland(t, "cleveland_empty.json", "cleveland_twilight.json")

	got, err := p.Search(context.Background(), media.Item{Title: "Twilight in the Wilderness", Creator: "F. E. Church"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if reqs[0].URL.Query().Get("artists") != "F. E. Church" {
		t.Errorf("first query %q, want the artist filter", reqs[0].URL.RawQuery)
	}
	if second := reqs[1].URL; second.Query().Has("artists") || second.Query().Get("title") != "Twilight in the Wilderness" || !hasFlag(second.RawQuery, "cc0") {
		t.Errorf("second query %q, want the title and cc0 without artists", second.RawQuery)
	}
	if len(got) != 1 || got[0].Title != "Twilight in the Wilderness" {
		t.Errorf("candidates = %+v", got)
	}
}

func TestClevelandSearchWithoutArtistMakesOneRequest(t *testing.T) {
	p, rs := fakeCleveland(t, "cleveland_empty.json")
	_, err := p.Search(context.Background(), media.Item{Title: "Nothing Like It"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if reqs := rs.requests(); len(reqs) != 1 || reqs[0].URL.Query().Has("artists") {
		t.Errorf("requests = %d, want one without artists", len(reqs))
	}
}

func TestClevelandSearchKeepsOnlyCC0WorksWithImages(t *testing.T) {
	p, _ := fakeCleveland(t, "cleveland_mixed.json")

	got, err := p.Search(context.Background(), media.Item{Title: "Twilight in the Wilderness", Creator: "Frederic Edwin Church"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The copyrighted study, the CC0 work with no images and the
	// non-matching title are dropped; a work with only a web image stays.
	want := []media.Candidate{{
		Title:     "Twilight in the Wilderness, sketch",
		Creator:   "Frederic Edwin Church",
		Year:      "1858",
		ImageURLs: []string{"https://openaccess-cdn.clevelandart.org/1900.3/1900.3_web.jpg"},
		Source:    "cleveland",
		Credit: "Title: Twilight in the Wilderness, sketch (c. 1858)\n" +
			"Artist: Frederic Edwin Church (American, 1826–1900)\n" +
			"Museum: The Cleveland Museum of Art\n" +
			"License: CC0 1.0 Universal (public domain), https://creativecommons.org/publicdomain/zero/1.0/\n" +
			"Object page: https://clevelandart.org/art/1900.3",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates =\n%+v\nwant\n%+v", got, want)
	}
}

func TestClevelandSearchSkipsWorksNotInThePublicDomain(t *testing.T) {
	p, _ := fakeCleveland(t, "cleveland_copyrighted.json")

	got, err := p.Search(context.Background(), media.Item{Title: "Composition in Blue", Creator: "Jane Painter"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("err = %v (candidates %+v), want ErrNotFound", err, got)
	}
	if msg := err.Error(); !strings.Contains(msg, "2 matches") || !strings.Contains(msg, "public-domain (CC0) image") {
		t.Errorf("err = %q, want it to say no match has a public-domain image", msg)
	}
}

func TestArtworkCreditOmitsEmptyFields(t *testing.T) {
	got := artworkCredit("Untitled", "", "", museumMet, "")
	want := "Title: Untitled\n" +
		"Museum: The Metropolitan Museum of Art\n" +
		"License: " + cc0License
	if got != want {
		t.Errorf("credit =\n%s\nwant\n%s", got, want)
	}
}

func TestStripLifeDates(t *testing.T) {
	for in, want := range map[string]string{
		"Frederic Edwin Church (American, 1826–1900)": "Frederic Edwin Church",
		"Claude Monet":                        "Claude Monet",
		"Workshop of Rubens (Flemish) (copy)": "Workshop of Rubens (Flemish)",
		"":                                    "",
	} {
		if got := stripLifeDates(in); got != want {
			t.Errorf("stripLifeDates(%q) = %q, want %q", in, got, want)
		}
	}
}

// The URL helper keeps the bare cc0 flag that url.Values cannot express.
func TestClevelandQueryKeepsBareFlag(t *testing.T) {
	q := clevelandQuery(url.Values{"title": {"A & B"}})
	if !hasFlag(q, "cc0") || !strings.Contains(q, "title=A+%26+B") {
		t.Errorf("query = %q", q)
	}
}

func TestMetSearchReportsTransientObjectErrors(t *testing.T) {
	// The first lookup fails with a server error and the last one is a 404:
	// the outage must not be reported as "not found".
	var rs routedServer
	srv := rs.serve(t, func(r *http.Request) (int, []byte) {
		switch r.URL.Path {
		case metSearchPath:
			return http.StatusOK, []byte(`{"total":2,"objectIDs":[1,2]}`)
		case metObjectPath + "1":
			return http.StatusServiceUnavailable, []byte(`{}`)
		}
		return http.StatusNotFound, []byte(`{"message":"ObjectID not found"}`)
	})
	p := NewMet(srv.Client())
	p.BaseURL = srv.URL + "/public/collection"

	_, err := p.Search(context.Background(), media.Item{Title: "Wheat Field with Cypresses"}, media.Options{})
	if err == nil || errors.Is(err, media.ErrNotFound) {
		t.Fatalf("err = %v, want the HTTP 503 error, not ErrNotFound", err)
	}
	var status *httpStatusError
	if !errors.As(err, &status) || status.Status != http.StatusServiceUnavailable {
		t.Errorf("err = %v, want HTTP 503", err)
	}
}

func TestMetSearchKeepsTitleMatchesWhenTheFallbackFails(t *testing.T) {
	ids := []string{"742412", "437397"}
	for i := range metMaxObjects {
		ids = append(ids, fmt.Sprint(100+i))
	}
	var rs routedServer
	known := map[string][]byte{
		"742412": fixture(t, "met_object_742412.json"),
		"437397": fixture(t, "met_object_437397.json"),
	}
	srv := rs.serve(t, func(r *http.Request) (int, []byte) {
		switch {
		case r.URL.Path == metSearchPath && r.URL.Query().Get("title") == "true":
			return http.StatusOK, []byte(`{"total":12,"objectIDs":[` + strings.Join(ids, ",") + `]}`)
		case r.URL.Path == metSearchPath:
			return http.StatusInternalServerError, []byte(`{}`)
		case strings.HasPrefix(r.URL.Path, metObjectPath):
			if body, ok := known[strings.TrimPrefix(r.URL.Path, metObjectPath)]; ok {
				return http.StatusOK, body
			}
		}
		return http.StatusNotFound, []byte(`{"message":"ObjectID not found"}`)
	})
	p := NewMet(srv.Client())
	p.BaseURL = srv.URL + "/public/collection"

	got, err := p.Search(context.Background(), media.Item{Title: "Self-Portrait", Creator: "Rembrandt"}, media.Options{})
	if err != nil {
		t.Fatalf("err = %v, want the title matches", err)
	}
	if len(got) != 1 || got[0].Title != "Self-Portrait" {
		t.Errorf("candidates = %+v, want Rembrandt's Self-Portrait", got)
	}
}
