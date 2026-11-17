package providers

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// wikimediaFixtures are the bodies a fake Wikidata + Commons server returns,
// recorded from the live APIs (testdata/wikimedia) and trimmed.
type wikimediaFixtures struct {
	search, entities, countries, imageinfo []byte
}

func loadWikimediaFixtures(t *testing.T, name string) wikimediaFixtures {
	t.Helper()
	f := wikimediaFixtures{
		search:    fixture(t, "wikimedia/"+name+"_search.json"),
		entities:  fixture(t, "wikimedia/"+name+"_entities.json"),
		imageinfo: fixture(t, "wikimedia/"+name+"_imageinfo.json"),
		countries: []byte(`{"entities":{}}`),
	}
	if name != "japan" {
		f.countries = fixture(t, "wikimedia/"+name+"_countries.json")
	}
	return f
}

const (
	wikidataTestPath = "/wikidata/w/api.php"
	commonsTestPath  = "/commons/w/api.php"
)

// wikimediaRoute serves f: Wikidata search, entities (claims) and country
// labels (props=labels), and Commons imageinfo.
func wikimediaRoute(f wikimediaFixtures) func(r *http.Request) (int, []byte) {
	return func(r *http.Request) (int, []byte) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == wikidataTestPath && q.Get("list") == "search":
			return http.StatusOK, f.search
		case r.URL.Path == wikidataTestPath && q.Get("action") == "wbgetentities" && q.Get("props") == "labels":
			return http.StatusOK, f.countries
		case r.URL.Path == wikidataTestPath && q.Get("action") == "wbgetentities":
			return http.StatusOK, f.entities
		case r.URL.Path == commonsTestPath && q.Get("prop") == "imageinfo":
			return http.StatusOK, f.imageinfo
		}
		return http.StatusNotFound, []byte(`{}`)
	}
}

func newWikimediaTest(t *testing.T, route func(r *http.Request) (int, []byte)) (*Wikimedia, *routedServer) {
	t.Helper()
	rs := &routedServer{}
	srv := rs.serve(t, route)
	p := NewWikimedia(srv.Client())
	p.WikidataURL = srv.URL + wikidataTestPath
	p.CommonsURL = srv.URL + commonsTestPath
	return p, rs
}

// imageinfoTitles returns the titles= of every imageinfo request.
func imageinfoTitles(rs *routedServer) []string {
	var out []string
	for _, r := range rs.requests() {
		if r.URL.Query().Get("prop") == "imageinfo" {
			out = append(out, r.URL.Query().Get("titles"))
		}
	}
	return out
}

func TestWikimediaName(t *testing.T) {
	p := NewWikimedia(nil)
	if p.Name() != "wikimedia" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.WikidataURL != "https://www.wikidata.org/w/api.php" || p.CommonsURL != "https://commons.wikimedia.org/w/api.php" {
		t.Errorf("default URLs = %q, %q", p.WikidataURL, p.CommonsURL)
	}
}

func TestWikimediaLandmark(t *testing.T) {
	p, _ := newWikimediaTest(t, wikimediaRoute(loadWikimediaFixtures(t, "eiffel")))

	got, err := p.Search(context.Background(), media.Item{Title: "Eiffel Tower", Creator: "France"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	const thumb = "https://thumb.wikimedia.org/wikipedia/commons/thumb/a/a8/Tour_Eiffel_Wikimedia_Commons.jpg/"
	const utm = "-Tour_Eiffel_Wikimedia_Commons.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail"
	want := media.Candidate{
		Title:     "Eiffel Tower",
		Creator:   "France",
		ImageURLs: []string{thumb + "1920px" + utm, thumb + "1280px" + utm},
		Source:    "wikimedia",
		Credit: strings.Join([]string{
			"Title: Tour Eiffel Wikimedia Commons",
			"Author: Benh LIEU SONG",
			"Source: Own work",
			"License: Public domain",
			"File page: https://commons.wikimedia.org/wiki/File:Tour_Eiffel_Wikimedia_Commons.jpg",
			"Wikidata item: https://www.wikidata.org/wiki/Q243",
		}, "\n"),
	}
	if len(got) != 3 {
		t.Fatalf("candidates = %d, want 3 (the top French hits)", len(got))
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first =\n%+v\nwant\n%+v", got[0], want)
	}

	// A painting smaller than 1920 px: Commons returns the unscaled
	// original, so there is no 1280 fallback.
	if want := []string{"https://upload.wikimedia.org/wikipedia/commons/c/c4/Robert_Delaunay_-_Eiffel_Tower_-_1911_-_Solomon_R._Guggenheim_Museum.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail_unscaled"}; !reflect.DeepEqual(got[1].ImageURLs, want) {
		t.Errorf("second ImageURLs = %q, want %q", got[1].ImageURLs, want)
	}
	for _, line := range []string{"Title: Eiffel Tower", "Author: Robert Delaunay", "Source: https://www.guggenheim.org/artwork/1022", "License: Public domain"} {
		if !strings.Contains(got[1].Credit, line) {
			t.Errorf("second credit lacks %q:\n%s", line, got[1].Credit)
		}
	}

	// Attribution replaces the uploader's Artist table.
	for _, line := range []string{"Author: © Polymagou / Wikimedia Commons\n", "License: CC BY-SA 4.0, https://creativecommons.org/licenses/by-sa/4.0\n"} {
		if !strings.Contains(got[2].Credit, line) {
			t.Errorf("third credit lacks %q:\n%s", line, got[2].Credit)
		}
	}
}

func TestWikimediaRequests(t *testing.T) {
	p, rs := newWikimediaTest(t, wikimediaRoute(loadWikimediaFixtures(t, "eiffel")))

	if _, err := p.Search(context.Background(), media.Item{Title: " Eiffel Tower ", Creator: "France"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if len(reqs) != 4 {
		t.Fatalf("requests = %d, want 4 (search, entities, countries, imageinfo)", len(reqs))
	}
	checks := []map[string]string{
		{
			"action": "query", "list": "search", "srsearch": "Eiffel Tower haswbstatement:P18",
			"srnamespace": "0", "srlimit": "10", "srprop": "", "format": "json",
		},
		{
			"action": "wbgetentities", "ids": "Q243|Q83667184|Q28061050|Q133271195|Q162873|Q25223034|Q3522264|Q5348880|Q5348879|Q136523068",
			"props": "labels|descriptions|claims", "languages": "en", "languagefallback": "1", "format": "json",
		},
		{
			// The hits' countries, in first-seen order.
			"action": "wbgetentities", "ids": "Q142|Q183|Q30|Q212", "props": "labels",
			"languages": "en", "languagefallback": "1", "format": "json",
		},
		{
			"action": "query", "prop": "imageinfo",
			"titles":              "File:Tour Eiffel Wikimedia Commons.jpg|File:Robert Delaunay - Eiffel Tower - 1911 - Solomon R. Guggenheim Museum.jpg|File:Broca, tour Eiffel, Paris 1.jpg",
			"iiprop":              "url|size|extmetadata",
			"iiurlwidth":          "1920",
			"iiextmetadatafilter": "ObjectName|Artist|Credit|Attribution|LicenseShortName|LicenseUrl|AttributionRequired|Copyrighted",
			"format":              "json",
		},
	}
	for i, want := range checks {
		q := reqs[i].URL.Query()
		for key, v := range want {
			if q.Get(key) != v {
				t.Errorf("request %d: %s = %q, want %q", i, key, q.Get(key), v)
			}
		}
	}
	for i, r := range reqs {
		if ua := r.Header.Get("User-Agent"); ua != web.ContactUserAgent {
			t.Errorf("request %d: User-Agent = %q, want the contact UA", i, ua)
		}
	}
}

func TestWikimediaCountryPreference(t *testing.T) {
	const (
		cordobaSpain     = "File:Centro Histórico, 16.9 -- 2023 -- Córdoba, España.jpg"
		provinceSpain    = "File:Cordoba Capital - 013 (30621941971).jpg"
		universitySpain  = "File:Universidad de Córdoba (52018268584).jpg"
		cordobaArgentina = "File:Montaje CBA 9.jpg"
		provinceArg      = "File:Cordoba Province Montage 2.png"
		universityArg    = "File:Patiocolonialcordoba.jpg"
		cordobaMexico    = "File:CordobaPM.jpg"
	)
	tests := []struct {
		name, country string
		wantTitles    []string
	}{
		// Hits in the country first, ranked by label; then the others.
		{"Spain", "Spain", []string{cordobaSpain, provinceSpain, universitySpain}},
		{"case and accents ignored", "  SPÁIN ", []string{cordobaSpain, provinceSpain, universitySpain}},
		{"Argentina", "Argentina", []string{cordobaArgentina, provinceArg, universityArg}},
		// One Mexican hit, then the best labels of the rest.
		{"Mexico", "Mexico", []string{cordobaMexico, cordobaArgentina, cordobaSpain}},
		// No country, or one no hit is in: label rank, search order on ties.
		{"no country", "", []string{cordobaArgentina, cordobaSpain, cordobaMexico}},
		{"unmatched country", "Peru", []string{cordobaArgentina, cordobaSpain, cordobaMexico}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, rs := newWikimediaTest(t, wikimediaRoute(loadWikimediaFixtures(t, "cordoba")))
			// The imageinfo fixture only holds the Spanish files; the
			// requested titles show the ranking.
			_, _ = p.Search(context.Background(), media.Item{Title: "Córdoba", Creator: tt.country}, media.Options{})
			got := imageinfoTitles(rs)
			if want := strings.Join(tt.wantTitles, "|"); len(got) != 1 || got[0] != want {
				t.Errorf("imageinfo titles = %q, want %q", got, want)
			}
		})
	}
}

func TestWikimediaCountryCandidates(t *testing.T) {
	p, _ := newWikimediaTest(t, wikimediaRoute(loadWikimediaFixtures(t, "cordoba")))

	got, err := p.Search(context.Background(), media.Item{Title: "Cordoba", Creator: "Spain"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, c := range got {
		titles = append(titles, c.Title+" - "+c.Creator)
	}
	if want := []string{"Córdoba - Spain", "Córdoba Province - Spain", "University of Cordoba - Spain"}; !reflect.DeepEqual(titles, want) {
		t.Errorf("candidates = %q, want %q", titles, want)
	}
	for _, line := range []string{
		"Title: Centro Histórico, 16.9 -- 2023 -- Córdoba, España\n",
		"Author: Jose María Ligero Loarte\n",
		"Source: Own work\n",
		"License: CC BY-SA 4.0, https://creativecommons.org/licenses/by-sa/4.0\n",
		"Wikidata item: https://www.wikidata.org/wiki/Q5818",
	} {
		if !strings.Contains(got[0].Credit, line) {
			t.Errorf("credit lacks %q:\n%s", line, got[0].Credit)
		}
	}
	// A Flickr author keeps its plain text, the link is dropped.
	if !strings.Contains(got[2].Credit, "Author: Kent Wang from Barcelona, Spain\n") {
		t.Errorf("third credit:\n%s", got[2].Credit)
	}
}

func TestWikimediaFlagMode(t *testing.T) {
	const thumb = "https://thumb.wikimedia.org/wikipedia/commons/thumb/9/9e/Flag_of_Japan.svg/"
	const utm = "-Flag_of_Japan.svg.png?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail"
	for _, item := range []media.Item{
		{Title: "Flag of Japan"},
		{Title: "  FLAG OF japan "},
		{Title: "Japan flag"},
		// The country column is ignored in flag mode.
		{Title: "Flag of Japan", Creator: "France"},
	} {
		t.Run(item.Title+"/"+item.Creator, func(t *testing.T) {
			p, rs := newWikimediaTest(t, wikimediaRoute(loadWikimediaFixtures(t, "japan")))

			got, err := p.Search(context.Background(), item, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 {
				t.Fatalf("candidates = %d, want 1 (the best label match)", len(got))
			}
			c := got[0]
			if c.Title != "Flag of Japan" || c.Creator != "" || c.Source != "wikimedia" {
				t.Errorf("candidate = %+v", c)
			}
			if want := []string{thumb + "1920px" + utm, thumb + "1280px" + utm}; !reflect.DeepEqual(c.ImageURLs, want) {
				t.Errorf("ImageURLs = %q, want %q", c.ImageURLs, want)
			}
			for _, line := range []string{"Title: Flag of Japan\n", "Author: Various\n", "License: Public domain\n",
				"File page: https://commons.wikimedia.org/wiki/File:Flag_of_Japan.svg\n", "Wikidata item: https://www.wikidata.org/wiki/Q17"} {
				if !strings.Contains(c.Credit, line) {
					t.Errorf("credit lacks %q:\n%s", line, c.Credit)
				}
			}

			reqs := rs.requests()
			if len(reqs) != 3 {
				t.Fatalf("requests = %d, want 3 (search, entities, imageinfo; no country lookup)", len(reqs))
			}
			if q := reqs[0].URL.Query().Get("srsearch"); q != "japan haswbstatement:P41" && q != "Japan haswbstatement:P41" {
				t.Errorf("srsearch = %q", q)
			}
			// Japan's preferred P41, not the historical flag.
			if got := imageinfoTitles(rs); !reflect.DeepEqual(got, []string{"File:Flag of Japan.svg"}) {
				t.Errorf("imageinfo titles = %q", got)
			}
		})
	}
}

func TestWikimediaNotFound(t *testing.T) {
	f := loadWikimediaFixtures(t, "eiffel")
	tests := []struct {
		name   string
		item   media.Item
		modify func(*wikimediaFixtures)
	}{
		{"empty title", media.Item{Title: "  "}, nil},
		{"no search hits", media.Item{Title: "Nowhere"}, func(f *wikimediaFixtures) {
			f.search = []byte(`{"batchcomplete":"","query":{"search":[]}}`)
		}},
		{"hits without an image", media.Item{Title: "Eiffel Tower"}, func(f *wikimediaFixtures) {
			f.entities = []byte(`{"entities":{
				"Q243":{"id":"Q243","labels":{"en":{"value":"Eiffel Tower"}},"claims":{}},
				"Q162873":{"id":"Q162873","labels":{"en":{"value":"72 names"}},"claims":{"P18":[{"mainsnak":{"datavalue":{"value":"x.jpg"}},"rank":"deprecated"}]}}
			}}`)
		}},
		{"files missing on Commons", media.Item{Title: "Eiffel Tower"}, func(f *wikimediaFixtures) {
			f.imageinfo = []byte(`{"query":{"pages":{"-1":{"ns":6,"title":"File:Tour Eiffel Wikimedia Commons.jpg","missing":""}}}}`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff := f
			if tt.modify != nil {
				tt.modify(&ff)
			}
			p, rs := newWikimediaTest(t, wikimediaRoute(ff))
			got, err := p.Search(context.Background(), tt.item, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Fatalf("got %v, err = %v, want ErrNotFound", got, err)
			}
			if tt.name == "hits without an image" {
				if !strings.Contains(err.Error(), "no Wikidata places with an image") {
					t.Errorf("err = %v", err)
				}
				if titles := imageinfoTitles(rs); len(titles) != 0 {
					t.Errorf("imageinfo requested for %q", titles)
				}
			}
		})
	}
}

func TestWikimediaErrors(t *testing.T) {
	apiError := []byte(`{"error":{"code":"maxlag","info":"Waiting for a database server"}}`)
	f := loadWikimediaFixtures(t, "eiffel")
	tests := []struct {
		name   string
		status int
		modify func(*wikimediaFixtures)
		want   string
	}{
		{"search API error", http.StatusOK, func(f *wikimediaFixtures) { f.search = apiError }, "maxlag"},
		{"entities API error", http.StatusOK, func(f *wikimediaFixtures) { f.entities = apiError }, "maxlag"},
		{"countries API error", http.StatusOK, func(f *wikimediaFixtures) { f.countries = apiError }, "maxlag"},
		{"imageinfo API error", http.StatusOK, func(f *wikimediaFixtures) { f.imageinfo = apiError }, "maxlag"},
		{"HTTP error", http.StatusServiceUnavailable, nil, "HTTP 503"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff := f
			if tt.modify != nil {
				tt.modify(&ff)
			}
			route := wikimediaRoute(ff)
			p, _ := newWikimediaTest(t, func(r *http.Request) (int, []byte) {
				if tt.status != http.StatusOK {
					return tt.status, []byte(`{}`)
				}
				return route(r)
			})
			_, err := p.Search(context.Background(), media.Item{Title: "Eiffel Tower", Creator: "France"}, media.Options{})
			if err == nil || errors.Is(err, media.ErrNotFound) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want an error with %q, not ErrNotFound", err, tt.want)
			}
		})
	}
}

func TestFlagCountry(t *testing.T) {
	tests := []struct {
		title, want string
		ok          bool
	}{
		{"Flag of Japan", "Japan", true},
		{"flag of the United Kingdom", "United Kingdom", true},
		{"  FLAG OF  Peru ", "Peru", true},
		{"Japan flag", "Japan", true},
		{"South Korea FLAG", "South Korea", true},
		{"Eiffel Tower", "", false},
		{"Flagstaff", "", false},
		{"Flag of", "", false},
		{"flag", "", false},
		{"Flag Fen", "", false},
	}
	for _, tt := range tests {
		got, ok := flagCountry(tt.title)
		if got != tt.want || ok != tt.ok {
			t.Errorf("flagCountry(%q) = %q, %v; want %q, %v", tt.title, got, ok, tt.want, tt.ok)
		}
	}
}

func TestPlaceInCountry(t *testing.T) {
	tests := []struct {
		name        string
		country     string
		labels      []string
		description string
		want        bool
	}{
		{"label", "France", []string{"France"}, "", true},
		{"case and accents", "méxico", []string{"Mexico"}, "", true},
		{"second country", "Peru", []string{"Bolivia", "Peru"}, "", true},
		{"alias", "USA", []string{"United States"}, "", true},
		{"alias the other way", "United States of America", []string{"United States"}, "", true},
		{"UK alias", "uk", []string{"United Kingdom"}, "", true},
		{"description", "Spain", nil, "municipality of Spain", true},
		{"description word only", "Spa", nil, "municipality of Spain", false},
		{"other country", "France", []string{"Germany"}, "replica of the tower in Berlin", false},
		{"no country data", "France", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := placeInCountry(normalizeCountry(tt.country), tt.labels, tt.description); got != tt.want {
				t.Errorf("placeInCountry(%q, %q, %q) = %v, want %v", tt.country, tt.labels, tt.description, got, tt.want)
			}
		})
	}
}

func TestHTMLText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "Benh LIEU SONG", "Benh LIEU SONG"},
		{"link", `<a href="//commons.wikimedia.org/wiki/User:Benh" title="User:Benh">Benh LIEU SONG</a>`, "Benh LIEU SONG"},
		{"inline tags join", `<bdi><a href="x"><span title="French painter">Robert Delaunay</span></a></bdi>, 1911`, "Robert Delaunay, 1911"},
		{"entities", `Tom &amp; Jerry &lt;b&gt; &quot;q&quot; &#233;`, `Tom & Jerry <b> "q" é`},
		{"block tags separate", "<p>Line one</p><p>Line two<br>three</p><ul><li>a</li><li>b</li></ul>", "Line one Line two three a b"},
		{"whitespace collapsed", "  a\n\t b   c ", "a b c"},
		{"table cells", "<table><tr><td>a</td><td>b</td></tr></table>", "a b"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := htmlText(tt.in); got != tt.want {
				t.Errorf("htmlText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCommonsLicense(t *testing.T) {
	tests := []struct {
		name                    string
		short, url, copyrighted string
		want                    string
	}{
		{"CC BY-SA", "CC BY-SA 4.0", "https://creativecommons.org/licenses/by-sa/4.0", "True", "CC BY-SA 4.0, https://creativecommons.org/licenses/by-sa/4.0"},
		{"public domain", "Public domain", "", "False", "Public domain"},
		{"CC0 not copyrighted", "CC0", "http://creativecommons.org/publicdomain/zero/1.0/deed.en", "False", "CC0 (public domain), http://creativecommons.org/publicdomain/zero/1.0/deed.en"},
		{"only not copyrighted", "", "", "False", "Public domain"},
		{"unknown", "", "", "", "unknown, see the file page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commonsLicense(tt.short, tt.url, tt.copyrighted); got != tt.want {
				t.Errorf("commonsLicense = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommonsCreditCapsLongFields(t *testing.T) {
	long := strings.Repeat("word ", 200)
	credit := commonsCredit(commonsImage{
		DescriptionURL: "https://commons.wikimedia.org/wiki/File:X.jpg",
		ExtMetadata: map[string]commonsMetadataValue{
			"Artist":           {Value: "Ann"},
			"Credit":           {Value: "<p>" + long + "</p>"},
			"LicenseShortName": {Value: "CC BY 4.0"},
		},
	}, "Fallback label", "Q1")
	lines := strings.Split(credit, "\n")
	if lines[0] != "Title: Fallback label" || lines[1] != "Author: Ann" {
		t.Errorf("credit:\n%s", credit)
	}
	if !strings.HasPrefix(lines[2], "Source: word word") || !strings.HasSuffix(lines[2], "…") || len([]rune(lines[2])) > len("Source: ")+commonsFieldLimit+1 {
		t.Errorf("source line not capped: %q", lines[2])
	}
}

// A credit equal to the author is not repeated.
func TestCommonsCreditSkipsCreditEqualToAuthor(t *testing.T) {
	credit := commonsCredit(commonsImage{ExtMetadata: map[string]commonsMetadataValue{
		"Artist": {Value: "<b>Ann</b>"},
		"Credit": {Value: "Ann"},
	}}, "X", "Q1")
	if strings.Contains(credit, "Source:") {
		t.Errorf("credit:\n%s", credit)
	}
}
