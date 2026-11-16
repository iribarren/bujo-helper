package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"cover-downloader/internal/media"
)

const (
	testIGDBImageBase  = "https://igdb-images.example/igdb/image/upload"
	testSteamImageBase = "https://steam-cdn.example/steam/apps"
)

func igdbCreds(id, secret string) func() (string, string) {
	return func() (string, string) { return id, secret }
}

// igdbImageURLs returns the IGDB cover URLs for imageID, best first.
func igdbImageURLs(imageID string) []string {
	return []string{
		testIGDBImageBase + "/t_1080p/" + imageID + ".jpg",
		testIGDBImageBase + "/t_720p/" + imageID + ".jpg",
		testIGDBImageBase + "/t_cover_big_2x/" + imageID + ".jpg",
	}
}

// fakeIGDB serves the Twitch token endpoint and the IGDB games endpoint.
// Tokens are handed out in order (the last one repeats); the games
// endpoint answers 401 to tokens listed in rejected.
type fakeIGDB struct {
	t         *testing.T
	tokens    []string
	expiresIn int
	// tokenStatus, if set, is the token endpoint's error status.
	tokenStatus int
	rejected    map[string]bool
	games       []byte

	mu          sync.Mutex
	tokenReqs   []*http.Request
	tokenForms  []url.Values
	gamesReqs   []*http.Request
	gamesBodies []string
}

func (f *fakeIGDB) serve() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/oauth2/token":
			form, _ := url.ParseQuery(string(body))
			f.tokenReqs = append(f.tokenReqs, r)
			f.tokenForms = append(f.tokenForms, form)
			if f.tokenStatus != 0 {
				w.WriteHeader(f.tokenStatus)
				io.WriteString(w, `{"status":403,"message":"invalid client secret"}`)
				return
			}
			var tok map[string]any
			if err := json.Unmarshal(fixture(f.t, "igdb_token.json"), &tok); err != nil {
				f.t.Fatal(err)
			}
			n := min(len(f.tokenReqs), len(f.tokens)) - 1
			tok["access_token"] = f.tokens[n]
			if f.expiresIn != 0 {
				tok["expires_in"] = f.expiresIn
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tok)
		case "/v4/games":
			f.gamesReqs = append(f.gamesReqs, r)
			f.gamesBodies = append(f.gamesBodies, string(body))
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if f.rejected[token] {
				w.WriteHeader(http.StatusUnauthorized)
				io.WriteString(w, `{"message":"Authorization Failure. Have you tried:","Tip 1":"Ensure you are sending Authorization and Client-ID as headers."}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(f.games)
		default:
			http.NotFound(w, r)
		}
	}))
	f.t.Cleanup(srv.Close)
	return srv
}

func (f *fakeIGDB) counts() (tokens, games int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tokenReqs), len(f.gamesReqs)
}

func newFakeIGDB(t *testing.T, games string, tokens ...string) *fakeIGDB {
	t.Helper()
	if len(tokens) == 0 {
		tokens = []string{"tok-1"}
	}
	return &fakeIGDB{t: t, tokens: tokens, games: fixture(t, games)}
}

func newTestIGDB(srv *httptest.Server, creds func() (string, string)) *IGDB {
	p := NewIGDB(srv.Client(), creds)
	p.BaseURL = srv.URL + "/v4"
	p.TokenURL = srv.URL + "/oauth2/token"
	p.ImageBase = testIGDBImageBase
	return p
}

func TestIGDBWithoutCredentialsSkipsWithoutHTTP(t *testing.T) {
	f := newFakeIGDB(t, "igdb_games.json")
	srv := f.serve()
	for name, creds := range map[string]func() (string, string){
		"nil credentials func": nil,
		"both empty":           igdbCreds("", ""),
		"no secret":            igdbCreds("client-id", ""),
		"no client ID":         igdbCreds(" ", "secret"),
	} {
		t.Run(name, func(t *testing.T) {
			p := newTestIGDB(srv, creds)
			_, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight", Creator: "2017"}, media.Options{})
			if !errors.Is(err, ErrNoIGDBCredentials) {
				t.Errorf("err = %v, want ErrNoIGDBCredentials", err)
			}
			if errors.Is(err, media.ErrNotFound) {
				t.Error("missing credentials must not wrap ErrNotFound")
			}
		})
	}
	if tokens, games := f.counts(); tokens+games != 0 {
		t.Errorf("server calls = %d token + %d games, want 0", tokens, games)
	}
}

func TestIGDBSearch(t *testing.T) {
	f := newFakeIGDB(t, "igdb_games.json")
	p := newTestIGDB(f.serve(), igdbCreds(" client-id ", " s3cret "))

	if p.Name() != "igdb" {
		t.Errorf("Name = %q, want igdb", p.Name())
	}
	got, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight", Creator: "2017"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	// The token request sends the credentials in the form body, not the URL.
	if len(f.tokenReqs) != 1 {
		t.Fatalf("token requests = %d, want 1", len(f.tokenReqs))
	}
	tr := f.tokenReqs[0]
	if tr.Method != http.MethodPost || tr.URL.RawQuery != "" {
		t.Errorf("token request = %s ?%s, want a POST without a query", tr.Method, tr.URL.RawQuery)
	}
	if ct := tr.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("token Content-Type = %q", ct)
	}
	wantForm := url.Values{"client_id": {"client-id"}, "client_secret": {"s3cret"}, "grant_type": {"client_credentials"}}
	if !reflect.DeepEqual(f.tokenForms[0], wantForm) {
		t.Errorf("token form = %v, want %v", f.tokenForms[0], wantForm)
	}

	gr := f.gamesReqs[0]
	if gr.Method != http.MethodPost {
		t.Errorf("games method = %s, want POST", gr.Method)
	}
	for header, want := range map[string]string{
		"Client-ID":     "client-id",
		"Authorization": "Bearer tok-1",
		"Accept":        "application/json",
		"User-Agent":    "cover-downloader/1.0",
	} {
		if got := gr.Header.Get(header); got != want {
			t.Errorf("games header %s = %q, want %q", header, got, want)
		}
	}
	wantBody := `search "Hollow Knight"; fields name,first_release_date,cover.image_id; where cover != null; limit 20;`
	if f.gamesBodies[0] != wantBody {
		t.Errorf("games body = %q, want %q", f.gamesBodies[0], wantBody)
	}

	// Results without a cover are dropped; the exact title ranks first.
	wantTitles := []string{"Hollow Knight", "Hollow Knight: Silksong", "Hollow Knight: Voidheart Edition"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:     "Hollow Knight",
		Creator:   "2017",
		Year:      "2017",
		ImageURLs: igdbImageURLs("co1rgi"),
		Source:    "igdb",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
	// A game without a release date has no year.
	if got[2].Year != "" || got[2].Creator != "" {
		t.Errorf("Voidheart Edition year = %q, want none", got[2].Year)
	}
}

func TestIGDBRanksByYear(t *testing.T) {
	for _, tt := range []struct {
		year      string
		wantImage string
	}{
		{"2017", "co1niz"},
		{"2006", "co1nhc"},
		{"", "co1nhc"}, // no year: the response order stands
	} {
		t.Run("year "+tt.year, func(t *testing.T) {
			f := newFakeIGDB(t, "igdb_games_prey.json")
			p := newTestIGDB(f.serve(), igdbCreds("id", "secret"))
			got, err := p.Search(context.Background(), media.Item{Title: "Prey", Creator: tt.year}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[0].ImageURLs, igdbImageURLs(tt.wantImage)) {
				t.Errorf("first ImageURLs = %q, want cover %s", got[0].ImageURLs, tt.wantImage)
			}
			if last := got[len(got)-1].Title; last != "Prey: Mooncrash" {
				t.Errorf("last = %q, want the inexact title last", last)
			}
		})
	}
}

func TestIGDBPrefersTheExactTitle(t *testing.T) {
	// The response lists the sequel first, and no year is given.
	f := newFakeIGDB(t, "igdb_games.json")
	p := newTestIGDB(f.serve(), igdbCreds("id", "secret"))
	got, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "Hollow Knight" {
		t.Errorf("first = %q, want the exact title before its sequel", got[0].Title)
	}
}

func TestIGDBSearchBodyEscapesTheTitle(t *testing.T) {
	for _, tt := range []struct {
		title, want string
	}{
		{`Hollow Knight`, `search "Hollow Knight";`},
		{` Say "Hi" `, `search "Say \"Hi\"";`},
		{`Back\Slash`, `search "Back\\Slash";`},
		{`a\"; limit 500;`, `search "a\\\"; limit 500;";`},
	} {
		t.Run(tt.title, func(t *testing.T) {
			f := newFakeIGDB(t, "igdb_games.json")
			p := newTestIGDB(f.serve(), igdbCreds("id", "secret"))
			if _, err := p.Search(context.Background(), media.Item{Title: tt.title}, media.Options{}); err != nil {
				t.Fatal(err)
			}
			if body := f.gamesBodies[0]; !strings.HasPrefix(body, tt.want+" fields ") {
				t.Errorf("body = %q, want it to start with %q", body, tt.want)
			}
		})
	}
}

func TestIGDBReusesTheToken(t *testing.T) {
	f := newFakeIGDB(t, "igdb_games.json", "tok-1", "tok-2", "tok-3")
	f.expiresIn = 3600
	p := newTestIGDB(f.serve(), nil)
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	id, secret := "id-a", "secret-a"
	p.Credentials = func() (string, string) { return id, secret }

	search := func() {
		t.Helper()
		if _, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight"}, media.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	lastAuth := func() string {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.gamesReqs[len(f.gamesReqs)-1].Header.Get("Authorization")
	}

	search()
	search()
	if tokens, games := f.counts(); tokens != 1 || games != 2 {
		t.Fatalf("calls = %d token + %d games, want 1 + 2", tokens, games)
	}

	// Shortly before expiry the token is refreshed.
	now = now.Add(3600*time.Second - 30*time.Second)
	search()
	if tokens, _ := f.counts(); tokens != 2 || lastAuth() != "Bearer tok-2" {
		t.Errorf("near expiry: tokens = %d, auth %q, want a second token", tokens, lastAuth())
	}

	// Other credentials get their own token.
	secret = "secret-b"
	search()
	if tokens, _ := f.counts(); tokens != 3 || lastAuth() != "Bearer tok-3" {
		t.Errorf("new secret: tokens = %d, auth %q, want a third token", tokens, lastAuth())
	}
	if got := f.tokenForms[2].Get("client_secret"); got != "secret-b" {
		t.Errorf("third token secret = %q, want secret-b", got)
	}
}

func TestIGDBRefreshesTheTokenOnceOn401(t *testing.T) {
	f := newFakeIGDB(t, "igdb_games.json", "stale", "fresh")
	f.rejected = map[string]bool{"stale": true}
	p := newTestIGDB(f.serve(), igdbCreds("id", "secret"))

	got, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight", Creator: "2017"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "Hollow Knight" {
		t.Errorf("first = %q, want Hollow Knight", got[0].Title)
	}
	if tokens, games := f.counts(); tokens != 2 || games != 2 {
		t.Errorf("calls = %d token + %d games, want 2 + 2", tokens, games)
	}
	if f.gamesBodies[0] != f.gamesBodies[1] {
		t.Errorf("retry body = %q, want the same query %q", f.gamesBodies[1], f.gamesBodies[0])
	}

	// The fresh token is kept for the next search.
	if _, err := p.Search(context.Background(), media.Item{Title: "Celeste"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	if tokens, games := f.counts(); tokens != 2 || games != 3 {
		t.Errorf("calls after a second search = %d token + %d games, want 2 + 3", tokens, games)
	}
}

func TestIGDBRejectedCredentials(t *testing.T) {
	t.Run("401 twice", func(t *testing.T) {
		f := newFakeIGDB(t, "igdb_games.json", "bad-1", "bad-2", "bad-3")
		f.rejected = map[string]bool{"bad-1": true, "bad-2": true, "bad-3": true}
		p := newTestIGDB(f.serve(), igdbCreds("id", "s3cret-value"))

		_, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight"}, media.Options{})
		if err == nil || err.Error() != "IGDB rejected the credentials (HTTP 401)" {
			t.Errorf("err = %v, want the rejected-credentials error", err)
		}
		if tokens, games := f.counts(); tokens != 2 || games != 2 {
			t.Errorf("calls = %d token + %d games, want one retry (2 + 2)", tokens, games)
		}
	})
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(fmt.Sprintf("token endpoint answers %d", status), func(t *testing.T) {
			f := newFakeIGDB(t, "igdb_games.json")
			f.tokenStatus = status
			p := newTestIGDB(f.serve(), igdbCreds("id", "secret"))

			_, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight"}, media.Options{})
			if err == nil || strings.Contains(err.Error(), "rejected") {
				t.Errorf("err = %v, want a token error that doesn't blame the credentials", err)
			}
		})
	}
	t.Run("token endpoint refuses", func(t *testing.T) {
		f := newFakeIGDB(t, "igdb_games.json")
		f.tokenStatus = http.StatusForbidden
		p := newTestIGDB(f.serve(), igdbCreds("id", "s3cret-value"))

		_, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight"}, media.Options{})
		if err == nil || err.Error() != "Twitch rejected the IGDB credentials (HTTP 403)" {
			t.Errorf("err = %v, want the rejected-credentials error", err)
		}
		if errors.Is(err, media.ErrNotFound) {
			t.Error("rejected credentials must not wrap ErrNotFound")
		}
		if _, games := f.counts(); games != 0 {
			t.Errorf("games calls = %d, want 0 without a token", games)
		}
	})
}

func TestIGDBTransportErrorHidesTheSecret(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	p := newTestIGDB(srv, igdbCreds("id", "s3cret-value"))
	srv.Close()

	_, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight"}, media.Options{})
	if err == nil {
		t.Fatal("want a transport error from a closed server")
	}
	if strings.Contains(err.Error(), "s3cret-value") {
		t.Errorf("error leaks the client secret: %v", err)
	}
}

func TestIGDBNotFound(t *testing.T) {
	for name, title := range map[string]string{"blank title": "  ", "no games": "Nothing"} {
		t.Run(name, func(t *testing.T) {
			f := newFakeIGDB(t, "igdb_games.json")
			f.games = []byte("[]")
			p := newTestIGDB(f.serve(), igdbCreds("id", "secret"))
			_, err := p.Search(context.Background(), media.Item{Title: title}, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

// fakeSteam serves the store search and the app details. details maps an
// app ID to its fixture; other IDs answer 500.
type fakeSteam struct {
	t       *testing.T
	search  []byte
	details map[string]string

	mu          sync.Mutex
	searchReqs  []*http.Request
	detailsReqs []*http.Request
}

func (f *fakeSteam) serve() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/storesearch/":
			f.searchReqs = append(f.searchReqs, r)
			w.Write(f.search)
		case "/api/appdetails":
			f.detailsReqs = append(f.detailsReqs, r)
			name, ok := f.details[r.URL.Query().Get("appids")]
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(fixture(f.t, name))
		default:
			http.NotFound(w, r)
		}
	}))
	f.t.Cleanup(srv.Close)
	return srv
}

func newTestSteam(t *testing.T, search string, details map[string]string) (*Steam, *fakeSteam) {
	t.Helper()
	f := &fakeSteam{t: t, search: fixture(t, search), details: details}
	srv := f.serve()
	p := NewSteam(srv.Client())
	p.BaseURL = srv.URL + "/api"
	p.ImageBase = testSteamImageBase
	return p, f
}

// steamImageURLs returns the Steam library capsule URLs for appID, best first.
func steamImageURLs(appID string) []string {
	base := testSteamImageBase + "/" + appID
	return []string{base + "/library_600x900_2x.jpg", base + "/library_600x900.jpg"}
}

func TestSteamSearch(t *testing.T) {
	p, f := newTestSteam(t, "steam_storesearch.json", nil)

	if p.Name() != "steam" {
		t.Errorf("Name = %q, want steam", p.Name())
	}
	got, err := p.Search(context.Background(), media.Item{Title: "Hollow Knight", Creator: "2017"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	r := f.searchReqs[0]
	for key, want := range map[string]string{"term": "Hollow Knight", "l": "english", "cc": "US"} {
		if got := r.URL.Query().Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}
	// Packages (type "sub") are dropped; the exact title ranks first.
	wantTitles := []string{"Hollow Knight", "Hollow Knight: Silksong", "Hollow Knight - Official Soundtrack"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:     "Hollow Knight",
		ImageURLs: steamImageURLs("367520"),
		Source:    "steam",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
	// A single top title needs no release-date lookup.
	if len(f.detailsReqs) != 0 {
		t.Errorf("appdetails calls = %d, want 0", len(f.detailsReqs))
	}
}

func TestSteamYearBreaksTitleTies(t *testing.T) {
	details := map[string]string{"3970": "steam_appdetails_3970.json", "480490": "steam_appdetails_480490.json"}
	for _, tt := range []struct {
		name        string
		year        string
		details     map[string]string
		wantFirst   string
		wantYear    string
		wantLookups int
	}{
		{"requested year wins", "2017", details, "480490", "2017", 2},
		{"first already matches", "2006", details, "3970", "2006", 2},
		{"no year skips the lookup", "", details, "3970", "", 0},
		{"lookup failures keep the order", "2017", nil, "3970", "", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestSteam(t, "steam_storesearch_prey.json", tt.details)
			got, err := p.Search(context.Background(), media.Item{Title: "Prey", Creator: tt.year}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[0].ImageURLs, steamImageURLs(tt.wantFirst)) {
				t.Errorf("first ImageURLs = %q, want app %s", got[0].ImageURLs, tt.wantFirst)
			}
			if got[0].Year != tt.wantYear {
				t.Errorf("first year = %q, want %q", got[0].Year, tt.wantYear)
			}
			if len(got) != 3 || got[2].Title != "Prey: Mooncrash" {
				t.Errorf("titles = %q, want the inexact title last", candidateTitles(got))
			}
			if len(f.detailsReqs) != tt.wantLookups {
				t.Errorf("appdetails calls = %d, want %d", len(f.detailsReqs), tt.wantLookups)
			}
			for _, r := range f.detailsReqs {
				if r.URL.Query().Get("filters") != "release_date" {
					t.Errorf("appdetails query = %q, want filters=release_date", r.URL.RawQuery)
				}
			}
		})
	}
}

func TestSteamNotFound(t *testing.T) {
	for name, title := range map[string]string{"blank title": " ", "no apps": "Nothing"} {
		t.Run(name, func(t *testing.T) {
			p, _ := newTestSteam(t, "steam_storesearch_empty.json", nil)
			_, err := p.Search(context.Background(), media.Item{Title: title}, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestSteamYearFromReleaseDate(t *testing.T) {
	for in, want := range map[string]string{
		"24 Feb, 2017": "2017",
		"Feb 24, 2017": "2017",
		"2017":         "2017",
		"Coming soon":  "",
		"":             "",
		"Q3 20256":     "",
	} {
		if got := steamYear(in); got != want {
			t.Errorf("steamYear(%q) = %q, want %q", in, got, want)
		}
	}
}
