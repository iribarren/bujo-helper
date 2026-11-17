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

// The BHL fixtures (testdata/bhl) are hand-made from the JSON examples of
// the BHL API v3 documentation (GetNameMetadata, GetItemMetadata), with
// invented titles, items and page IDs: a recording needs an API key.
// unauthorized.json is the live answer to a request without a key.

const (
	bhlTestPath = "/api3"
	bhlTestKey  = "12345678-BBBB-DDDD-FFFF-123456789012"
)

// bhlFixtures are the BHL bodies a fake server returns: the name metadata
// and the item metadata by item ID. The Wikidata lookup uses the recorded
// robin fixtures.
type bhlFixtures struct {
	taxa  taxonFixtures
	name  []byte
	items map[string][]byte
	// status, if set, answers every BHL request with it and body.
	status int
	body   []byte
}

func loadBHLFixtures(t *testing.T) *bhlFixtures {
	t.Helper()
	return &bhlFixtures{
		taxa: loadTaxonFixtures(t, "robin"),
		name: fixture(t, "bhl/robin_name.json"),
		items: map[string][]byte{
			"201": fixture(t, "bhl/item_201_public_domain.json"),
			"301": fixture(t, "bhl/item_301_no_known_restrictions.json"),
		},
	}
}

func newBHLTest(t *testing.T, f *bhlFixtures) (*BHL, *routedServer) {
	t.Helper()
	rs := &routedServer{}
	srv := rs.serve(t, func(r *http.Request) (int, []byte) {
		if status, body, ok := f.taxa.route(r); ok {
			return status, body
		}
		if r.URL.Path != bhlTestPath {
			return http.StatusNotFound, []byte(`{}`)
		}
		if f.status != 0 {
			return f.status, f.body
		}
		q := r.URL.Query()
		switch q.Get("op") {
		case "GetNameMetadata":
			return http.StatusOK, f.name
		case "GetItemMetadata":
			if body, ok := f.items[q.Get("id")]; ok {
				return http.StatusOK, body
			}
			return http.StatusOK, []byte(`{"Status":"ok","ErrorMessage":"","Result":[]}`)
		}
		return http.StatusBadRequest, []byte(`{"Status":"error","ErrorMessage":"unknown op"}`)
	})
	taxa := NewTaxonResolver(srv.Client())
	taxa.WikidataURL = srv.URL + wikidataTestPath
	p := NewBHL(srv.Client(), func() string { return bhlTestKey }, taxa)
	p.APIURL = srv.URL + bhlTestPath
	return p, rs
}

// bhlRequests returns "op id" for every BHL request, in order.
func bhlRequests(rs *routedServer) []string {
	var out []string
	for _, r := range rs.requests() {
		if r.URL.Path != bhlTestPath {
			continue
		}
		q := r.URL.Query()
		out = append(out, strings.TrimSpace(q.Get("op")+" "+q.Get("id")))
	}
	return out
}

func TestBHLDefaults(t *testing.T) {
	p := NewBHL(nil, nil, nil)
	if p.Client == nil || p.Taxa == nil || p.APIURL != "https://www.biodiversitylibrary.org/api3" || p.MaxResponseBytes != 16<<20 {
		t.Errorf("NewBHL(nil, nil, nil) = %+v", p)
	}
	if p.Name() != "bhl" {
		t.Errorf("Name = %q", p.Name())
	}
}

func TestBHLWithoutAKeyMakesNoRequest(t *testing.T) {
	for name, key := range map[string]func() string{
		"nil key":   nil,
		"blank key": func() string { return "  " },
	} {
		t.Run(name, func(t *testing.T) {
			p, rs := newBHLTest(t, loadBHLFixtures(t))
			p.Key = key
			_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
			if !errors.Is(err, ErrNoBHLKey) || errors.Is(err, media.ErrNotFound) {
				t.Errorf("err = %v, want ErrNoBHLKey", err)
			}
			if n := len(rs.requests()); n != 0 {
				t.Errorf("requests = %d, want none", n)
			}
		})
	}
}

func TestBHLPublicDomainPlates(t *testing.T) {
	p, rs := newBHLTest(t, loadBHLFixtures(t))
	got, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The oldest title (1885) first; its illustration page and its plate,
	// not its text page.
	want := []media.Candidate{
		{
			Title:     "European robin",
			Creator:   "Erithacus rubecula",
			ImageURLs: []string{"https://www.biodiversitylibrary.org/pageimage/20012"},
			Source:    "bhl",
			Credit: strings.Join([]string{
				"Image from the Biodiversity Heritage Library. Contributed by Natural History Museum Library, London. | www.biodiversitylibrary.org",
				"Title: Coloured figures of the birds of the British Islands, v.1",
				"Date: 1885-1897",
				"Page: https://www.biodiversitylibrary.org/page/20012",
				"Copyright status: NOT_IN_COPYRIGHT",
				"Item: https://www.biodiversitylibrary.org/item/201",
				"Wikidata taxon: https://www.wikidata.org/wiki/Q25334",
			}, "\n"),
		},
		{
			Title:     "European robin",
			Creator:   "Erithacus rubecula",
			ImageURLs: []string{"https://www.biodiversitylibrary.org/pageimage/20013"},
			Source:    "bhl",
			Credit: strings.Join([]string{
				"Image from the Biodiversity Heritage Library. Contributed by Natural History Museum Library, London. | www.biodiversitylibrary.org",
				"Title: Coloured figures of the birds of the British Islands, v.1",
				"Date: 1885-1897",
				"Page: https://www.biodiversitylibrary.org/page/20013 (Plate 4)",
				"Copyright status: NOT_IN_COPYRIGHT",
				"Item: https://www.biodiversitylibrary.org/item/201",
				"Wikidata taxon: https://www.wikidata.org/wiki/Q25334",
			}, "\n"),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Search =\n%+v\nwant\n%+v", got, want)
	}
	if reqs, want := bhlRequests(rs), []string{"GetNameMetadata", "GetItemMetadata 201"}; !reflect.DeepEqual(reqs, want) {
		t.Errorf("BHL requests = %q, want %q", reqs, want)
	}
	// The name request: the taxon name, JSON, the key.
	for _, r := range rs.requests() {
		if r.URL.Path != bhlTestPath {
			continue
		}
		q := r.URL.Query()
		if q.Get("format") != "json" || q.Get("apikey") != bhlTestKey {
			t.Errorf("query = %v, want format=json and the key", q)
		}
		if q.Get("op") == "GetNameMetadata" && q.Get("name") != "Erithacus rubecula" {
			t.Errorf("name = %q, want the taxon name", q.Get("name"))
		}
		if q.Get("op") == "GetItemMetadata" && q.Get("idtype") != "bhl" {
			t.Errorf("idtype = %q, want bhl", q.Get("idtype"))
		}
		if ua := r.Header.Get("User-Agent"); ua != web.ContactUserAgent {
			t.Errorf("User-Agent = %q, want the contact UA", ua)
		}
	}
}

func TestBHLSkipsItemsThatAreNotPublicDomain(t *testing.T) {
	t.Run("next item used", func(t *testing.T) {
		f := loadBHLFixtures(t)
		f.items["201"] = fixture(t, "bhl/item_201_in_copyright.json")
		p, rs := newBHLTest(t, f)
		got, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !reflect.DeepEqual(got[0].ImageURLs, []string{"https://www.biodiversitylibrary.org/pageimage/30011"}) {
			t.Fatalf("Search = %+v, want the 1923 illustration", got)
		}
		for _, line := range []string{
			"Contributed by Smithsonian Libraries and Archives.",
			"Copyright status: No known copyright restrictions as determined by scanning institution.",
			"Rights: https://www.biodiversitylibrary.org/permissions",
		} {
			if !strings.Contains(got[0].Credit, line) {
				t.Errorf("credit lacks %q:\n%s", line, got[0].Credit)
			}
		}
		if reqs, want := bhlRequests(rs), []string{"GetNameMetadata", "GetItemMetadata 201", "GetItemMetadata 301"}; !reflect.DeepEqual(reqs, want) {
			t.Errorf("BHL requests = %q, want %q", reqs, want)
		}
	})
	t.Run("all skipped", func(t *testing.T) {
		f := loadBHLFixtures(t)
		f.items["201"] = fixture(t, "bhl/item_201_in_copyright.json")
		f.items["301"] = fixture(t, "bhl/item_301_public_domain_nc.json")
		p, _ := newBHLTest(t, f)
		_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
		if !errors.Is(err, media.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		for _, want := range []string{
			"no public-domain BHL illustration of Erithacus rubecula",
			"Coloured figures of the birds of the British Islands (item 201): in copyright (In copyright. Digitized with the permission of the rights holder.)",
			"Journal of a garden naturalist (item 301): non-commercial licence (http://creativecommons.org/licenses/by-nc/3.0/)",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("err = %v\nwant it to contain %q", err, want)
			}
		}
	})
}

func TestBHLNotFound(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(t *testing.T, f *bhlFixtures)
		wantErr string
		wantBHL []string
	}{
		{"text pages only", func(t *testing.T, f *bhlFixtures) {
			f.name = fixture(t, "bhl/text_only_name.json")
		}, "BHL has no illustration or plate naming Erithacus rubecula (2 pages)", []string{"GetNameMetadata"}},
		{"name not in BHL", func(t *testing.T, f *bhlFixtures) {
			f.name = []byte(`{"Status":"ok","ErrorMessage":"","Result":[]}`)
		}, "BHL has no illustration or plate naming Erithacus rubecula (0 pages)", []string{"GetNameMetadata"}},
		{"taxon without a scientific name", func(t *testing.T, f *bhlFixtures) {
			f.taxa.claims = []byte(`{"entities":{"Q25334":{"id":"Q25334","claims":{}}}}`)
		}, "European robin (Q25334) has no taxon name (P225)", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadBHLFixtures(t)
			tt.modify(t, f)
			p, rs := newBHLTest(t, f)
			_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
			if !errors.Is(err, media.ErrNotFound) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want ErrNotFound with %q", err, tt.wantErr)
			}
			if reqs := bhlRequests(rs); !reflect.DeepEqual(reqs, tt.wantBHL) {
				t.Errorf("BHL requests = %q, want %q", reqs, tt.wantBHL)
			}
		})
	}
}

func TestBHLErrorsHideTheKey(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    func(t *testing.T) []byte
		wantErr string
	}{
		{"no or wrong key", http.StatusUnauthorized, func(t *testing.T) []byte { return fixture(t, "bhl/unauthorized.json") },
			"BHL rejected the API key (HTTP 401)"},
		{"unauthorized status", http.StatusOK, func(*testing.T) []byte {
			return []byte(`{"Status":"unauthorized","ErrorMessage":"'` + bhlTestKey + `' is an invalid or unauthorized API key.","Result":null}`)
		}, "BHL rejected the API key (HTTP 401)"},
		{"API error quoting the key", http.StatusOK, func(*testing.T) []byte {
			return []byte(`{"Status":"error","ErrorMessage":"key ` + bhlTestKey + ` failed","Result":null}`)
		}, "BHL API error: key REDACTED failed"},
		{"server error", http.StatusInternalServerError, func(*testing.T) []byte { return []byte(`<html>oops</html>`) },
			"HTTP 500"},
		{"not JSON", http.StatusOK, func(*testing.T) []byte { return []byte(`<html>challenge</html>`) },
			"decode BHL GetNameMetadata response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadBHLFixtures(t)
			f.status, f.body = tt.status, tt.body(t)
			p, _ := newBHLTest(t, f)
			_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
			if err == nil || errors.Is(err, media.ErrNotFound) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), bhlTestKey) {
				t.Errorf("err %q shows the key", err)
			}
		})
	}

	t.Run("transport error", func(t *testing.T) {
		p, _ := newBHLTest(t, loadBHLFixtures(t))
		// Resolve the taxon first, then point the API at a closed port.
		if _, err := p.Taxa.Resolve(context.Background(), "Robin"); err != nil {
			t.Fatal(err)
		}
		p.APIURL = "http://127.0.0.1:1/api3"
		_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
		if err == nil || strings.Contains(err.Error(), bhlTestKey) || !strings.Contains(err.Error(), "REDACTED") {
			t.Errorf("err = %v, want a transport error with the key redacted", err)
		}
	})
}

func TestBHLOversizedResponseFailsCleanly(t *testing.T) {
	p, rs := newBHLTest(t, loadBHLFixtures(t))
	p.MaxResponseBytes = 1 << 10
	_, err := p.Search(context.Background(), media.Item{Title: "Robin"}, media.Options{})
	if !errors.Is(err, web.ErrBodyTooLarge) || errors.Is(err, media.ErrNotFound) {
		t.Fatalf("err = %v, want web.ErrBodyTooLarge", err)
	}
	if !strings.Contains(err.Error(), "GetNameMetadata") || strings.Contains(err.Error(), bhlTestKey) {
		t.Errorf("err = %v, want it to name the call and hide the key", err)
	}
	if reqs, want := bhlRequests(rs), []string{"GetNameMetadata"}; !reflect.DeepEqual(reqs, want) {
		t.Errorf("BHL requests = %q, want %q", reqs, want)
	}
}

func TestBHLRights(t *testing.T) {
	tests := []struct {
		status, license, want string
	}{
		{"NOT_IN_COPYRIGHT", "", ""},
		{"Not in copyright", "", ""},
		{"Public domain. The BHL considers that this work is no longer under copyright protection.", "", ""},
		{"Not specified", "", ""},
		{"", "", ""},
		{"No known copyright restrictions as determined by scanning institution.", "", ""},
		{"Public domain", "http://creativecommons.org/publicdomain/mark/1.0/", ""},
		{"In copyright. Digitized with the permission of the rights holder.", "http://creativecommons.org/licenses/by-nc-sa/3.0/", "in copyright (In copyright. Digitized with the permission of the rights holder.)"},
		{"Permission to digitize granted by rights holder", "", "not public domain (Permission to digitize granted by rights holder)"},
		{"NOT_IN_COPYRIGHT", "https://creativecommons.org/licenses/by-nd/4.0/", "no-derivatives licence (https://creativecommons.org/licenses/by-nd/4.0/)"},
	}
	for _, tt := range tests {
		t.Run(tt.status+" "+tt.license, func(t *testing.T) {
			if got := bhlRights(tt.status, tt.license); got != tt.want {
				t.Errorf("bhlRights(%q, %q) = %q, want %q", tt.status, tt.license, got, tt.want)
			}
		})
	}
}
