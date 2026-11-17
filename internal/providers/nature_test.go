package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// The Commons searches of the nature provider for the recorded taxa.
const (
	robinDepicts     = "haswbstatement:P180=Q25334 haswbstatement:P6216=Q19652 filetype:bitmap"
	robinCategory    = `incategory:"Erithacus rubecula - botanical illustrations"`
	robinPlate       = `"Erithacus rubecula" plate filetype:bitmap`
	monsteraDepicts  = "haswbstatement:P180=Q161077 haswbstatement:P6216=Q19652 filetype:bitmap"
	monsteraCategory = `incategory:"Monstera deliciosa - botanical illustrations"`
	monsteraPlate    = `"Monstera deliciosa" plate filetype:bitmap`

	lilfordRobin  = "File:Coloured figures of the birds of the British Islands - issued by Lord Lilford (10036178774).jpg"
	berlinerPlant = "File:Berliner allgemeine Gartenzeitung (19755855571).jpg"
)

// natureFixtures are the bodies a fake Wikidata + Commons server returns,
// recorded live (testdata/nature) and trimmed. Commons searches not in
// searches find nothing; imageinfo holds every recorded file and the
// server answers with the requested ones only.
type natureFixtures struct {
	taxon     taxonFixtures
	searches  map[string][]byte
	imageinfo []byte
}

func loadNatureFixtures(t *testing.T, name string) natureFixtures {
	t.Helper()
	f := natureFixtures{
		taxon:     loadTaxonFixtures(t, name),
		imageinfo: fixture(t, "nature/"+name+"_commons_imageinfo.json"),
	}
	switch name {
	case "robin":
		f.searches = map[string][]byte{
			robinDepicts: fixture(t, "nature/robin_commons_depicts.json"),
			robinPlate:   fixture(t, "nature/robin_commons_plate.json"),
		}
	case "monstera":
		f.searches = map[string][]byte{
			monsteraDepicts:  fixture(t, "nature/monstera_commons_depicts.json"),
			monsteraCategory: fixture(t, "nature/monstera_commons_category.json"),
		}
	}
	return f
}

const emptyCommonsSearch = `{"batchcomplete":"","query":{"search":[]}}`

func natureRoute(t *testing.T, f natureFixtures) func(r *http.Request) (int, []byte) {
	return func(r *http.Request) (int, []byte) {
		if status, body, ok := f.taxon.route(r); ok {
			return status, body
		}
		q := r.URL.Query()
		switch {
		case r.URL.Path == commonsTestPath && q.Get("list") == "search":
			if body, ok := f.searches[q.Get("srsearch")]; ok {
				return http.StatusOK, body
			}
			return http.StatusOK, []byte(emptyCommonsSearch)
		case r.URL.Path == commonsTestPath && q.Get("prop") == "imageinfo":
			return http.StatusOK, onlyPages(t, f.imageinfo, strings.Split(q.Get("titles"), "|"))
		}
		return http.StatusNotFound, []byte(`{}`)
	}
}

// onlyPages keeps the imageinfo pages whose title is in titles, as Commons
// answers. A body without pages (an API error) is returned unchanged.
func onlyPages(t *testing.T, body []byte, titles []string) []byte {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	query, ok := resp["query"].(map[string]any)
	if !ok {
		return body
	}
	want := map[string]bool{}
	for _, title := range titles {
		want[title] = true
	}
	pages, _ := query["pages"].(map[string]any)
	for id, page := range pages {
		if title, _ := page.(map[string]any)["title"].(string); !want[title] {
			delete(pages, id)
		}
	}
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// withLicense returns the imageinfo fixture body with the licence fields of
// the files titles (every file when titles is empty) replaced; an empty
// value removes the field.
func withLicense(t *testing.T, body []byte, short, licenseURL, copyrighted string, titles ...string) []byte {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	only := map[string]bool{}
	for _, title := range titles {
		only[title] = true
	}
	for _, page := range resp["query"].(map[string]any)["pages"].(map[string]any) {
		page := page.(map[string]any)
		if len(only) > 0 && !only[page["title"].(string)] {
			continue
		}
		meta := page["imageinfo"].([]any)[0].(map[string]any)["extmetadata"].(map[string]any)
		for field, v := range map[string]string{"LicenseShortName": short, "LicenseUrl": licenseURL, "Copyrighted": copyrighted} {
			if v == "" {
				delete(meta, field)
			} else {
				meta[field] = map[string]any{"value": v}
			}
		}
	}
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func newNatureTest(t *testing.T, f natureFixtures) (*CommonsNature, *routedServer) {
	t.Helper()
	rs := &routedServer{}
	srv := rs.serve(t, natureRoute(t, f))
	p := NewCommonsNature(srv.Client(), nil)
	p.Taxa.WikidataURL = srv.URL + wikidataTestPath
	p.CommonsURL = srv.URL + commonsTestPath
	return p, rs
}

// commonsSearches returns the srsearch of every Commons search request.
func commonsSearches(rs *routedServer) []string {
	var out []string
	for _, r := range rs.requests() {
		if r.URL.Path == commonsTestPath && r.URL.Query().Get("list") == "search" {
			out = append(out, r.URL.Query().Get("srsearch"))
		}
	}
	return out
}

func natureCandidates(cs []media.Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Title+" | "+c.Credit[:strings.IndexByte(c.Credit, '\n')])
	}
	return out
}

func TestCommonsNatureDefaults(t *testing.T) {
	p := NewCommonsNature(nil, nil)
	if p.Name() != "commons-nature" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.Client == nil || p.Taxa == nil || p.CommonsURL != "https://commons.wikimedia.org/w/api.php" {
		t.Errorf("defaults = %+v", p)
	}
	shared := NewTaxonResolver(nil)
	if NewCommonsNature(nil, shared).Taxa != shared {
		t.Error("the given resolver is not used")
	}
}

func TestCommonsNatureBird(t *testing.T) {
	p, _ := newNatureTest(t, loadNatureFixtures(t, "robin"))

	got, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := media.Candidate{
		Title:     "European robin",
		Creator:   "Erithacus rubecula",
		ImageURLs: []string{"https://upload.wikimedia.org/wikipedia/commons/0/05/Coloured_figures_of_the_birds_of_the_British_Islands_-_issued_by_Lord_Lilford_%2810036178774%29.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail_unscaled"},
		Source:    "commons-nature",
		Credit: strings.Join([]string{
			"Title: Coloured figures of the birds of the British Islands - issued by Lord Lilford (10036178774)",
			"Author: Keulemans, J. G.; Lilford, Thomas Littleton Powys; Newton, Alfred; Salvin, Osbert; Thorburn, Archibald",
			// The hidden "date QS:…" part of the date is dropped.
			"Date: between 1885 and 1897",
			"Source: https://www.flickr.com/photos/biodivlibrary/10036178774",
			"License: Public domain",
			"File page: https://commons.wikimedia.org/wiki/File:Coloured_figures_of_the_birds_of_the_British_Islands_-_issued_by_Lord_Lilford_(10036178774).jpg",
			"Wikidata taxon: https://www.wikidata.org/wiki/Q25334",
			"Image from the Biodiversity Heritage Library",
		}, "\n"),
	}
	// The BHL plate ranks first; the rest keep the search order.
	if len(got) != 3 {
		t.Fatalf("candidates = %d, want 3", len(got))
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first =\n%+v\nwant\n%+v", got[0], want)
	}
	for i, wantTitle := range []string{
		"Title: Aanschouwd deez' vogelen, ô jeugd!",
		"Title: De groote vogelkooi",
	} {
		c := got[i+1]
		if !strings.HasPrefix(c.Credit, wantTitle+"\n") || strings.Contains(c.Credit, "Biodiversity Heritage Library") {
			t.Errorf("candidate %d credit:\n%s", i+1, c.Credit)
		}
	}
}

func TestCommonsNatureRequests(t *testing.T) {
	p, rs := newNatureTest(t, loadNatureFixtures(t, "robin"))
	if _, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if len(reqs) != 5 {
		t.Fatalf("requests = %d, want 5 (taxon search, labels, claims, Commons search, imageinfo)", len(reqs))
	}
	checks := map[int]map[string]string{
		3: {
			"action": "query", "list": "search", "srsearch": robinDepicts,
			"srnamespace": "6", "srlimit": "10", "srprop": "", "format": "json",
		},
		4: {
			"action": "query", "prop": "imageinfo",
			"titles": strings.Join([]string{
				lilfordRobin,
				"File:Aanschouwd deez' vogelen, ô jeugd!-Catchpenny print-Borms 0397.jpeg",
				"File:De groote vogelkooi-Catchpenny print-Borms 0710.jpeg",
				"File:Der naturen bloeme - Jacob van Maerlant - KB KA 16 - 099r.jpg",
				"File:Nederlandsche vogelen (KB) - Erithacus rubecula (086b).jpg",
			}, "|"),
			"iiprop":              "url|size|extmetadata",
			"iiurlwidth":          "1920",
			"iiextmetadatafilter": "ObjectName|Artist|Credit|Attribution|LicenseShortName|LicenseUrl|AttributionRequired|Copyrighted|DateTimeOriginal",
			"format":              "json",
		},
	}
	for i, want := range checks {
		if reqs[i].URL.Path != commonsTestPath {
			t.Errorf("request %d path = %q", i, reqs[i].URL.Path)
		}
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

func TestCommonsNatureHouseplant(t *testing.T) {
	p, _ := newNatureTest(t, loadNatureFixtures(t, "monstera"))

	got, err := p.Search(context.Background(), media.Item{Title: "Monstera deliciosa"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := media.Candidate{
		// The label is the scientific name: no creator.
		Title:     "Monstera deliciosa",
		ImageURLs: []string{"https://upload.wikimedia.org/wikipedia/commons/e/e7/Berliner_allgemeine_Gartenzeitung_%2819755855571%29.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail_unscaled"},
		Source:    "commons-nature",
		Credit: strings.Join([]string{
			"Title: Berliner allgemeine Gartenzeitung (19755855571)",
			// The hidden duplicate of the author is dropped.
			"Author: Unknown author",
			"Date: 1857",
			"Source: https://www.flickr.com/photos/biodivlibrary/19755855571",
			"License: Public domain",
			"File page: https://commons.wikimedia.org/wiki/File:Berliner_allgemeine_Gartenzeitung_(19755855571).jpg",
			"Wikidata taxon: https://www.wikidata.org/wiki/Q161077",
			"Image from the Biodiversity Heritage Library",
		}, "\n"),
	}
	if len(got) != 3 {
		t.Fatalf("candidates = %d, want 3", len(got))
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first =\n%+v\nwant\n%+v", got[0], want)
	}
	// A BHL page credit (not Flickr) also counts as BHL.
	if !strings.HasPrefix(got[1].Credit, "Title: Vintage illustrations by Miss May Rivers digitally enhanced by rawpixel 11\n") ||
		!strings.Contains(got[1].Credit, "\nImage from the Biodiversity Heritage Library") {
		t.Errorf("second credit:\n%s", got[1].Credit)
	}
	wide := "https://thumb.wikimedia.org/wikipedia/commons/thumb/1/19/Vintage_illustrations_by_Miss_May_Rivers_digitally_enhanced_by_rawpixel_11.jpg/"
	utm := "-Vintage_illustrations_by_Miss_May_Rivers_digitally_enhanced_by_rawpixel_11.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail"
	if want := []string{wide + "1920px" + utm, wide + "1280px" + utm}; !reflect.DeepEqual(got[1].ImageURLs, want) {
		t.Errorf("second ImageURLs = %q, want %q", got[1].ImageURLs, want)
	}
}

func TestCommonsNatureStrategyFallthrough(t *testing.T) {
	tests := []struct {
		name, fixtures, query string
		drop                  string // the search emptied
		wantSearches          []string
		wantFirst             string
	}{
		{
			// No depicts hit, and the Commons category has no botanical
			// illustrations subcategory: the "plate" search, where every
			// hit is a BHL plate, so the search order stays.
			name: "bird to plate search", fixtures: "robin", query: "Robin", drop: robinDepicts,
			wantSearches: []string{robinDepicts, robinCategory, robinPlate},
			wantFirst:    "European robin | Title: A book of birds (Plate XXIX) (6021714941)",
		},
		{
			name: "plant to botanical illustrations", fixtures: "monstera", query: "Monstera deliciosa", drop: monsteraDepicts,
			wantSearches: []string{monsteraDepicts, monsteraCategory},
			wantFirst:    "Monstera deliciosa | Title: Berliner allgemeine Gartenzeitung (19755855571)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadNatureFixtures(t, tt.fixtures)
			delete(f.searches, tt.drop)
			p, rs := newNatureTest(t, f)
			got, err := p.Search(context.Background(), media.Item{Title: tt.query}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if s := commonsSearches(rs); !reflect.DeepEqual(s, tt.wantSearches) {
				t.Errorf("searches = %q, want %q", s, tt.wantSearches)
			}
			if titles := natureCandidates(got); len(titles) == 0 || titles[0] != tt.wantFirst {
				t.Errorf("candidates = %q, want first %q", titles, tt.wantFirst)
			}
		})
	}
}

func TestCommonsNatureRanking(t *testing.T) {
	const nederlandsche = "File:Nederlandsche vogelen (KB) - Erithacus rubecula (086b).jpg"
	tests := []struct {
		name      string
		hits      []string // the plate search hits, in search order
		noBHL     bool     // drop the BHL credits from the image info
		wantInfo  string   // the imageinfo titles requested
		wantFirst string
	}{
		{
			// "Plate" in the title ranks first, even before the image
			// info is fetched.
			name:      "plate in the title",
			hits:      []string{nederlandsche, "File:Familiar wild birds (Plate 14) (6032908456).jpg"},
			noBHL:     true,
			wantInfo:  "File:Familiar wild birds (Plate 14) (6032908456).jpg|" + nederlandsche,
			wantFirst: "European robin | Title: Familiar wild birds (Plate 14) (6032908456)",
		},
		{
			name:      "BHL credit",
			hits:      []string{nederlandsche, lilfordRobin},
			wantInfo:  nederlandsche + "|" + lilfordRobin,
			wantFirst: "European robin | Title: Coloured figures of the birds of the British Islands - issued by Lord Lilford (10036178774)",
		},
		{
			name:      "neither keeps the search order",
			hits:      []string{nederlandsche, lilfordRobin},
			noBHL:     true,
			wantInfo:  nederlandsche + "|" + lilfordRobin,
			wantFirst: "European robin | Title: European robin from Nederlandsche vogelen (Dutch birds) by Nozeman and Sepp (1770-1829)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadNatureFixtures(t, "robin")
			delete(f.searches, robinDepicts)
			var hits []string
			for _, h := range tt.hits {
				hits = append(hits, `{"ns":6,"title":`+strconvQuote(h)+`}`)
			}
			f.searches[robinPlate] = []byte(`{"query":{"search":[` + strings.Join(hits, ",") + `]}}`)
			if tt.noBHL {
				f.imageinfo = []byte(strings.ReplaceAll(string(f.imageinfo), "biodivlibrary", "someone"))
			}
			p, rs := newNatureTest(t, f)
			got, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if titles := natureCandidates(got); len(titles) != 2 || titles[0] != tt.wantFirst {
				t.Errorf("candidates = %q, want first %q", titles, tt.wantFirst)
			}
			if bhl := strings.Contains(got[0].Credit, "Biodiversity Heritage Library"); bhl == tt.noBHL {
				t.Errorf("BHL line = %v:\n%s", bhl, got[0].Credit)
			}
			if titles := imageinfoTitles(rs); len(titles) != 1 || titles[0] != tt.wantInfo {
				t.Errorf("imageinfo titles = %q, want %q", titles, tt.wantInfo)
			}
		})
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestCommonsNatureSkipsNonOpenLicences(t *testing.T) {
	t.Run("one non-commercial file", func(t *testing.T) {
		f := loadNatureFixtures(t, "robin")
		f.imageinfo = withLicense(t, f.imageinfo, "CC BY-NC 2.0", "https://creativecommons.org/licenses/by-nc/2.0", "True", lilfordRobin)
		p, _ := newNatureTest(t, f)
		got, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range got {
			if strings.Contains(c.Credit, "Lilford") {
				t.Errorf("the non-commercial file is a candidate:\n%s", c.Credit)
			}
		}
		if len(got) != 3 || !strings.HasPrefix(got[0].Credit, "Title: Aanschouwd") {
			t.Errorf("candidates = %q", natureCandidates(got))
		}
	})

	t.Run("nothing open", func(t *testing.T) {
		f := loadNatureFixtures(t, "monstera")
		f.imageinfo = withLicense(t, f.imageinfo, "CC BY-ND 4.0", "https://creativecommons.org/licenses/by-nd/4.0", "True")
		f.imageinfo = withLicense(t, f.imageinfo, "", "", "", berlinerPlant)
		p, rs := newNatureTest(t, f)
		got, err := p.Search(context.Background(), media.Item{Title: "Monstera deliciosa"}, media.Options{})
		if !errors.Is(err, media.ErrNotFound) {
			t.Fatalf("got %q, err = %v, want ErrNotFound", natureCandidates(got), err)
		}
		// Every strategy is tried before giving up.
		if s := commonsSearches(rs); !reflect.DeepEqual(s, []string{monsteraDepicts, monsteraCategory, monsteraPlate}) {
			t.Errorf("searches = %q", s)
		}
		for _, want := range []string{
			"no openly licensed Commons illustration of Monstera deliciosa (Q161077)",
			"Berliner allgemeine Gartenzeitung (19755855571).jpg: no licence information",
			"Monstera deliciosa.png: no-derivatives licence (CC BY-ND 4.0)",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("err lacks %q:\n%v", want, err)
			}
		}
	})
}

func TestCommonsNatureNotFound(t *testing.T) {
	tests := []struct {
		name    string
		item    media.Item
		modify  func(*natureFixtures)
		wantErr string
	}{
		{"empty name", media.Item{Title: " "}, nil, "empty name"},
		{"no taxon", media.Item{Title: "Robin"}, func(f *natureFixtures) {
			f.taxon.search = []byte(emptyCommonsSearch)
		}, "no Wikidata taxon matches"},
		{"no Commons files", media.Item{Title: "Robin"}, func(f *natureFixtures) {
			f.searches = nil
		}, "no openly licensed Commons illustration of European robin (Q25334)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadNatureFixtures(t, "robin")
			if tt.modify != nil {
				tt.modify(&f)
			}
			p, rs := newNatureTest(t, f)
			got, err := p.Search(context.Background(), tt.item, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Fatalf("got %q, err = %v, want ErrNotFound", natureCandidates(got), err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
			if titles := imageinfoTitles(rs); len(titles) != 0 {
				t.Errorf("imageinfo requested for %q", titles)
			}
		})
	}
}

func TestCommonsNatureErrors(t *testing.T) {
	apiError := []byte(`{"error":{"code":"maxlag","info":"Waiting for a database server"}}`)
	tests := []struct {
		name   string
		status int
		modify func(*natureFixtures)
		want   string
	}{
		{"taxon search API error", http.StatusOK, func(f *natureFixtures) { f.taxon.search = apiError }, "maxlag"},
		{"Commons search API error", http.StatusOK, func(f *natureFixtures) { f.searches[robinDepicts] = apiError }, "maxlag"},
		{"imageinfo API error", http.StatusOK, func(f *natureFixtures) { f.imageinfo = apiError }, "maxlag"},
		{"HTTP error", http.StatusServiceUnavailable, nil, "HTTP 503"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadNatureFixtures(t, "robin")
			if tt.modify != nil {
				tt.modify(&f)
			}
			route := natureRoute(t, f)
			rs := &routedServer{}
			srv := rs.serve(t, func(r *http.Request) (int, []byte) {
				if tt.status != http.StatusOK {
					return tt.status, []byte(`{}`)
				}
				return route(r)
			})
			p := NewCommonsNature(srv.Client(), nil)
			p.Taxa.WikidataURL = srv.URL + wikidataTestPath
			p.CommonsURL = srv.URL + commonsTestPath
			_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
			if err == nil || errors.Is(err, media.ErrNotFound) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want an error with %q, not ErrNotFound", err, tt.want)
			}
		})
	}
}

func TestOpenLicense(t *testing.T) {
	tests := []struct {
		name                    string
		short, url, copyrighted string
		wantReason              string // "" means accepted
	}{
		{"public domain", "Public domain", "", "False", ""},
		{"not copyrighted without a name", "", "", "False", ""},
		{"PD template", "PD-old-70", "", "", ""},
		{"CC0", "CC0", "http://creativecommons.org/publicdomain/zero/1.0/deed.en", "False", ""},
		{"CC BY", "CC BY 2.0", "https://creativecommons.org/licenses/by/2.0", "True", ""},
		{"CC BY-SA", "CC BY-SA 4.0", "https://creativecommons.org/licenses/by-sa/4.0", "True", ""},
		{"CC BY-SA template style", "cc-by-sa-3.0", "", "True", ""},
		{"non-commercial", "CC BY-NC 2.0", "https://creativecommons.org/licenses/by-nc/2.0", "True", "non-commercial licence (CC BY-NC 2.0)"},
		{"non-commercial share-alike", "CC BY-NC-SA 4.0", "", "True", "non-commercial licence (CC BY-NC-SA 4.0)"},
		{"no derivatives", "CC BY-ND 4.0", "", "True", "no-derivatives licence (CC BY-ND 4.0)"},
		{"non-commercial only in the URL", "Some licence", "https://creativecommons.org/licenses/by-nc-nd/4.0/", "True", "non-commercial licence (Some licence)"},
		{"other free licence", "GFDL", "http://www.gnu.org/copyleft/fdl.html", "True", "licence not accepted (GFDL)"},
		{"missing", "", "", "", "no licence information"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := openLicense(tt.short, tt.url, tt.copyrighted); got != tt.wantReason {
				t.Errorf("openLicense(%q, %q, %q) = %q, want %q", tt.short, tt.url, tt.copyrighted, got, tt.wantReason)
			}
		})
	}
}

func TestVisibleHTMLText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"hidden date data", `between 1885 and 1897<div style="display: none;">date QS:P,+1850-00-00T00:00:00Z/7</div>`, "between 1885 and 1897"},
		{"hidden duplicate", `Unknown author<span style="display: none;">Unknown author</span>`, "Unknown author"},
		{"visible tags kept", `<bdi><a href="x">May Rivers</a></bdi>`, "May Rivers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visibleHTMLText(tt.in); got != tt.want {
				t.Errorf("visibleHTMLText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
