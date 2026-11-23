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

// offProvider returns an Open Food Facts provider whose product and search
// APIs are a routed test server; images keep the real host, so candidate
// URLs are the real ones.
func offProvider(t *testing.T, route func(r *http.Request) (int, []byte)) (*OpenFoodFacts, *routedServer) {
	t.Helper()
	rs := &routedServer{}
	srv := rs.serve(t, route)
	p := NewOpenFoodFacts(srv.Client())
	p.WorldURL = srv.URL
	p.SearchURL = srv.URL
	return p, rs
}

// offFixture serves one testdata/openfoodfacts fixture for every request.
func offFixture(t *testing.T, name string) func(*http.Request) (int, []byte) {
	body := fixture(t, "openfoodfacts/"+name)
	return func(*http.Request) (int, []byte) { return http.StatusOK, body }
}

func TestOpenFoodFactsNameAndLabel(t *testing.T) {
	p := NewOpenFoodFacts(nil)
	if p.Name() != "openfoodfacts" || p.Label() != "Open Food Facts" {
		t.Errorf("Name, Label = %q, %q", p.Name(), p.Label())
	}
	if p.WorldURL != "https://world.openfoodfacts.org" || p.SearchURL != "https://search.openfoodfacts.org" ||
		p.ImagesURL != "https://images.openfoodfacts.org" || p.Client == nil {
		t.Errorf("defaults = %+v", p)
	}
}

func TestOFFBarcode(t *testing.T) {
	for _, tt := range []struct {
		name, title, creator, want string
		ok                         bool
	}{
		{"EAN-13", "3017620422003", "", "3017620422003", true},
		{"EAN-8", "18025211", "", "18025211", true},
		{"UPC-A with spaces", "0 12345 67890 5", "", "012345678905", true},
		{"GTIN-14 with hyphens", "1-0012345-678902", "", "10012345678902", true},
		{"surrounding spaces", "  3017620422003 ", "", "3017620422003", true},
		{"seven digits", "1234567", "", "", false},
		{"fifteen digits", "123456789012345", "", "", false},
		{"letters", "Nutella", "", "", false},
		{"digits and a letter", "301762042200X", "", "", false},
		{"non-ASCII digits", "٣٠١٧٦٢٠٤", "", "", false},
		{"brand set", "3017620422003", "Ferrero", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := offBarcode(media.Item{Title: tt.title, Creator: tt.creator})
			if got != tt.want || ok != tt.ok {
				t.Errorf("offBarcode(%q, %q) = %q, %v; want %q, %v", tt.title, tt.creator, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestOFFImageURL(t *testing.T) {
	for _, tt := range []struct{ code, want string }{
		{"18025211", "https://images.openfoodfacts.org/images/products/18025211/front_en.4.full.jpg"},
		{"1234", "https://images.openfoodfacts.org/images/products/1234/front_en.4.full.jpg"},
		{"012345678905", "https://images.openfoodfacts.org/images/products/001/234/567/8905/front_en.4.full.jpg"},
		{"3017620422003", "https://images.openfoodfacts.org/images/products/301/762/042/2003/front_en.4.full.jpg"},
		{"10012345678902", "https://images.openfoodfacts.org/images/products/100/123/456/78902/front_en.4.full.jpg"},
	} {
		if got := offImageURL("https://images.openfoodfacts.org", tt.code, "front_en", "4"); got != tt.want {
			t.Errorf("offImageURL(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestOFFQuery(t *testing.T) {
	for _, tt := range []struct{ name, title, brand, want string }{
		{"name only", "Nutella", "", "Nutella"},
		{"name and brand", "Nutella", "Ferrero", `Nutella brands:"Ferrero"`},
		{"trimmed", "  Earl Grey ", " Twinings ", `Earl Grey brands:"Twinings"`},
		{"lucene specials in the name", `a+b-c&&d||e!(f){g}[h]^"i"~j*k?l:m\n/o`,
			"", `a\+b\-c\&\&d\|\|e\!\(f\)\{g\}\[h\]\^\"i\"\~j\*k\?l\:m\\n\/o`},
		{"quote and backslash in the brand", "Tea", `Ben "&" Jerry\s`, `Tea brands:"Ben \"&\" Jerry\\s"`},
		{"brand only", "", "Acme", `brands:"Acme"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := offQuery(tt.title, tt.brand); got != tt.want {
				t.Errorf("offQuery(%q, %q) = %q, want %q", tt.title, tt.brand, got, tt.want)
			}
		})
	}
}

func TestOFFLanguage(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts media.Options
		want string
	}{
		{"MARC language", media.Options{Language: "spa"}, "es"},
		{"MARC wins over the country", media.Options{Language: "fre", Country: "es"}, "fr"},
		{"ISO 639-1 language", media.Options{Language: "DE"}, "de"},
		{"country only", media.Options{Country: "br"}, "pt"},
		{"unknown language", media.Options{Language: "xyz"}, ""},
		{"nothing", media.Options{}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := offLanguage(tt.opts); got != tt.want {
				t.Errorf("offLanguage(%+v) = %q, want %q", tt.opts, got, tt.want)
			}
		})
	}
}

// offImages builds a product images map; each front_<lang> has the given
// full size (0×0 leaves the sizes out).
func offImages(t *testing.T, fronts map[string][2]int) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for key, wh := range fronts {
		img := map[string]any{"imgid": "1", "rev": 7}
		if wh != [2]int{} {
			img["sizes"] = map[string]any{"full": map[string]int{"w": wh[0], "h": wh[1]}}
		}
		b, err := json.Marshal(img)
		if err != nil {
			t.Fatal(err)
		}
		out[key] = b
	}
	return out
}

func TestOFFFront(t *testing.T) {
	big := [2]int{800, 1000}
	for _, tt := range []struct {
		name     string
		fronts   map[string][2]int
		lang     string
		prodLang string
		key      string
		reason   string
	}{
		{"preferred language first", map[string][2]int{"front_en": big, "front_fr": big, "front_es": big}, "es", "fr", "front_es", ""},
		{"then the product language", map[string][2]int{"front_en": big, "front_fr": big}, "es", "fr", "front_fr", ""},
		{"then English", map[string][2]int{"front_en": big, "front_de": big}, "es", "fr", "front_en", ""},
		{"then any front, sorted", map[string][2]int{"front_pl": big, "front_de": big}, "es", "fr", "front_de", ""},
		{"no language known", map[string][2]int{"front_it": big}, "", "", "front_it", ""},
		{"small preferred front falls through", map[string][2]int{"front_es": {300, 900}, "front_en": big}, "es", "en", "front_en", ""},
		{"shorter side exactly 400", map[string][2]int{"front_en": {400, 600}}, "", "en", "front_en", ""},
		{"too small", map[string][2]int{"front_en": {300, 250}}, "", "en", "", "front image too small (300×250 px)"},
		{"size unknown", map[string][2]int{"front_en": {}}, "", "en", "", "front image size unknown"},
		{"first reason wins", map[string][2]int{"front_en": {}, "front_fr": {10, 10}}, "", "en", "", "front image size unknown"},
		{"no front", map[string][2]int{"ingredients_en": big}, "", "en", "", "no front image"},
		{"bare front key is not a language front", map[string][2]int{"front": big}, "", "en", "", "no front image"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := offProduct{Lang: tt.prodLang, Images: offImages(t, tt.fronts)}
			key, img, reason := p.front(tt.lang)
			if key != tt.key || reason != tt.reason {
				t.Fatalf("front(%q) = %q, %q; want %q, %q", tt.lang, key, reason, tt.key, tt.reason)
			}
			if key != "" && (img.Rev != "7" || img.ImgID != "1") {
				t.Errorf("image = %+v, want rev 7, imgid 1", img)
			}
		})
	}
}

func TestOFFBrands(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		want       offBrands
	}{
		{"comma-separated string", `"Ferrero, Nutella"`, offBrands{"Ferrero", "Nutella"}},
		{"array", `["Ferrero", " Nutella", ""]`, offBrands{"Ferrero", "Nutella"}},
		{"empty string", `""`, nil},
		{"null", `null`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got offBrands
			if err := json.Unmarshal([]byte(tt.body), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("brands = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOFFCredit(t *testing.T) {
	for _, tt := range []struct{ name, title, brand, uploader, want string }{
		{"all fields", "Nutella", "Ferrero", "kiliweb", "Title: Nutella (Ferrero)\n" +
			"Contributor: kiliweb\n" +
			"Source: Open Food Facts\n" +
			"License: CC BY-SA 3.0 (https://creativecommons.org/licenses/by-sa/3.0/)\n" +
			"Product page: https://world.openfoodfacts.org/product/3017620422003"},
		{"no brand or uploader", "Nutella", "", "", "Title: Nutella\n" +
			"Source: Open Food Facts\n" +
			"License: CC BY-SA 3.0 (https://creativecommons.org/licenses/by-sa/3.0/)\n" +
			"Product page: https://world.openfoodfacts.org/product/3017620422003"},
		{"line breaks folded", "Nut\nella", "Fer\r\nrero", "kili\nweb", "Title: Nut ella (Fer rero)\n" +
			"Contributor: kili web\n" +
			"Source: Open Food Facts\n" +
			"License: CC BY-SA 3.0 (https://creativecommons.org/licenses/by-sa/3.0/)\n" +
			"Product page: https://world.openfoodfacts.org/product/3017620422003"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := offCredit(tt.title, tt.brand, tt.uploader, "3017620422003"); got != tt.want {
				t.Errorf("credit =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestOpenFoodFactsSearchByNameAndBrand(t *testing.T) {
	p, rs := offProvider(t, offFixture(t, "search_nutella_ferrero.json"))
	got, err := p.Search(context.Background(), media.Item{Title: "Nutella", Creator: "Ferrero"}, media.Options{Language: "spa"})
	if err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	r := reqs[0]
	q := r.URL.Query()
	if r.URL.Path != "/search" || q.Get("q") != `Nutella brands:"Ferrero"` || q.Get("page_size") != "10" ||
		q.Get("fields") != "code,product_name,brands,lang,images" || q.Get("langs") != "es,en" {
		t.Errorf("request = %s", r.URL)
	}
	if ua := r.Header.Get("User-Agent"); ua != web.ContactUserAgent {
		t.Errorf("User-Agent = %q, want the contact UA", ua)
	}
	// Exact name matches first, in search order; the hit without a front
	// image is skipped and only three candidates are kept.
	want := []string{
		"https://images.openfoodfacts.org/images/products/385/989/351/2032/front_hr.6.full.jpg",
		"https://images.openfoodfacts.org/images/products/800/050/027/8802/front_en.33.full.jpg",
		"https://images.openfoodfacts.org/images/products/18025211/front_en.3.full.jpg",
	}
	var urls []string
	for _, c := range got {
		if len(c.ImageURLs) != 1 {
			t.Errorf("%s: ImageURLs = %q, want one", c.Title, c.ImageURLs)
		}
		urls = append(urls, c.ImageURLs...)
	}
	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("image URLs =\n%q\nwant\n%q", urls, want)
	}
	c := got[1]
	if c.Title != "Nutella" || c.Creator != "Ferrero" || c.Source != "openfoodfacts" {
		t.Errorf("candidate = %+v", c)
	}
	if want := offCredit("Nutella", "Ferrero", "niki77", "8000500278802"); c.Credit != want {
		t.Errorf("credit =\n%s\nwant\n%s", c.Credit, want)
	}
}

func TestOpenFoodFactsSearchByNameOnly(t *testing.T) {
	p, rs := offProvider(t, offFixture(t, "search_nutella_ferrero.json"))
	if _, err := p.Search(context.Background(), media.Item{Title: "Nutella"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	q := rs.requests()[0].URL.Query()
	if q.Get("q") != "Nutella" || q.Has("langs") {
		t.Errorf("query = %v, want q=Nutella and no langs", q)
	}
}

func TestOpenFoodFactsBarcodeLookup(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts media.Options
		key  string
		who  string
	}{
		{"preferred language", media.Options{Language: "spa"}, "front_es.621", "thaialagata"},
		{"small preferred front falls back to the product language", media.Options{Language: "ger"}, "front_en.879", "org-ferrero-france-commerciale"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, rs := offProvider(t, offFixture(t, "product_3017620422003.json"))
			got, err := p.Search(context.Background(), media.Item{Title: "3017620422003"}, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			r := rs.requests()[0]
			if r.URL.Path != "/api/v2/product/3017620422003" || r.URL.Query().Get("fields") != "code,product_name,brands,lang,images" {
				t.Errorf("request = %s", r.URL)
			}
			if ua := r.Header.Get("User-Agent"); ua != web.ContactUserAgent {
				t.Errorf("User-Agent = %q, want the contact UA", ua)
			}
			if len(got) != 1 {
				t.Fatalf("candidates = %d, want 1", len(got))
			}
			c := got[0]
			wantURL := "https://images.openfoodfacts.org/images/products/301/762/042/2003/" + tt.key + ".full.jpg"
			if !reflect.DeepEqual(c.ImageURLs, []string{wantURL}) {
				t.Errorf("ImageURLs = %q, want %q", c.ImageURLs, wantURL)
			}
			if c.Title != "Nutella" || c.Creator != "Ferrero" {
				t.Errorf("Title, Creator = %q, %q", c.Title, c.Creator)
			}
			if want := offCredit("Nutella", "Ferrero", tt.who, "3017620422003"); c.Credit != want {
				t.Errorf("credit =\n%s\nwant\n%s", c.Credit, want)
			}
		})
	}
}

func TestOpenFoodFactsNotFound(t *testing.T) {
	tiny := `{"status":1,"product":{"code":"3017620422003","product_name":"Nutella","brands":"Ferrero",` +
		`"images":{"front_en":{"imgid":"1","rev":"2","sizes":{"full":{"w":300,"h":250}}}}}}`
	for _, tt := range []struct {
		name   string
		item   media.Item
		status int
		body   []byte
		want   []string
	}{
		{"unknown barcode (status 0)", media.Item{Title: "0000000000017"}, http.StatusOK,
			fixture(t, "openfoodfacts/product_not_found.json"), []string{"no Open Food Facts product with barcode 0000000000017"}},
		{"unknown barcode (404)", media.Item{Title: "3017620422003"}, http.StatusNotFound,
			[]byte(`{"status":0}`), []string{"no Open Food Facts product with barcode 3017620422003"}},
		{"barcode product with a small front", media.Item{Title: "3017620422003"}, http.StatusOK,
			[]byte(tiny), []string{"Nutella (3017620422003): front image too small (300×250 px)"}},
		{"no search hits", media.Item{Title: "Nonexistent", Creator: "Nobody"}, http.StatusOK,
			fixture(t, "openfoodfacts/search_empty.json"), []string{`no Open Food Facts product matches "Nonexistent" by Nobody`}},
		{"every hit skipped", media.Item{Title: "Tea", Creator: "Acme"}, http.StatusOK,
			fixture(t, "openfoodfacts/search_skipped.json"), []string{
				"Tiny Tea (12345678): front image too small (300×250 px)",
				"Bare Tea (4000000000002): no front image",
				"Sizeless Tea (4000000000019): front image size unknown",
				"Odd Tea (abc): invalid barcode",
			}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := offProvider(t, func(*http.Request) (int, []byte) { return tt.status, tt.body })
			got, err := p.Search(context.Background(), tt.item, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Fatalf("Search = %v, %v; want ErrNotFound", got, err)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestOpenFoodFactsHTTPErrors(t *testing.T) {
	for _, item := range []media.Item{{Title: "3017620422003"}, {Title: "Nutella", Creator: "Ferrero"}} {
		p, _ := offProvider(t, func(*http.Request) (int, []byte) { return http.StatusServiceUnavailable, nil })
		_, err := p.Search(context.Background(), item, media.Options{})
		var se *httpStatusError
		if !errors.As(err, &se) || se.Status != http.StatusServiceUnavailable || errors.Is(err, media.ErrNotFound) {
			t.Errorf("%q: err = %v, want HTTP 503", item.Title, err)
		}
	}
}
