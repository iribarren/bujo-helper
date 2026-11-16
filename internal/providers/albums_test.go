package providers

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

const testCoverBase = "https://caa.example/release-group"

func newTestMusicBrainz(rec *recorder, t *testing.T, status int, body []byte) *MusicBrainz {
	t.Helper()
	srv := rec.serve(t, status, "application/json", body)
	p := NewMusicBrainz(srv.Client())
	p.BaseURL = srv.URL + "/ws/2/release-group/"
	p.CoverBase = testCoverBase
	return p
}

// caaURLs returns the Cover Art Archive front image URLs, best first.
func caaURLs(mbid string) []string {
	base := testCoverBase + "/" + mbid
	return []string{base + "/front-1200", base + "/front-500", base + "/front"}
}

func TestMusicBrainzSearch(t *testing.T) {
	var rec recorder
	p := newTestMusicBrainz(&rec, t, 200, fixture(t, "musicbrainz_release_groups.json"))

	if p.Name() != "musicbrainz" {
		t.Errorf("Name = %q, want musicbrainz", p.Name())
	}
	got, err := p.Search(context.Background(), media.Item{Title: "Abbey Road", Creator: "The Beatles"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	if rec.last.URL.Path != "/ws/2/release-group/" {
		t.Errorf("path = %q, want /ws/2/release-group/", rec.last.URL.Path)
	}
	q := rec.last.URL.Query()
	for key, want := range map[string]string{
		"query": `releasegroup:"Abbey Road" AND artist:"The Beatles"`,
		"fmt":   "json",
		"limit": "25",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	// MusicBrainz requires a User-Agent with contact information.
	const wantUA = "cover-downloader/1.0 ( https://github.com/iribarren/bullet-journal-covers )"
	if ua := rec.last.Header.Get("User-Agent"); ua != wantUA || web.MusicBrainzUserAgent != wantUA {
		t.Errorf("User-Agent = %q (constant %q), want %q", ua, web.MusicBrainzUserAgent, wantUA)
	}

	// Albums come before other release groups, then the title ranking
	// decides; non-album groups are kept.
	wantTitles := []string{
		"Abbey Road",
		"Abbey Road Remix",
		"Abbey Road / Hey Jude",
		"Abbey Road Companion",
		"The Alternate Abbey Road",
		"The Alternate Abbey Road",
	}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:     "Abbey Road",
		Creator:   "The Beatles",
		Year:      "1969",
		ImageURLs: caaURLs("9162580e-5df4-32de-80cc-f45a8d8a9b1d"),
		Source:    "musicbrainz",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
	// A release group without a release date has no year.
	if got[3].Year != "" {
		t.Errorf("Abbey Road Companion year = %q, want none", got[3].Year)
	}
}

func TestMusicBrainzPrefersAlbumsOnTies(t *testing.T) {
	var rec recorder
	p := newTestMusicBrainz(&rec, t, 200, fixture(t, "musicbrainz_release_groups.json"))

	// The response lists the "Other" release group before the album with
	// the same title.
	got, err := p.Search(context.Background(), media.Item{Title: "The Alternate Abbey Road", Creator: "The Beatles"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("candidates = %d, want 6", len(got))
	}
	for i, mbid := range []string{"827951f1-205f-4949-8e78-79dc57e20a30", "1624c19c-c4c5-31fd-96fe-b0c77def7a7b"} {
		if !reflect.DeepEqual(got[i].ImageURLs, caaURLs(mbid)) {
			t.Errorf("candidate %d ImageURLs = %q, want release group %s", i, got[i].ImageURLs, mbid)
		}
	}
}

func TestMusicBrainzQuery(t *testing.T) {
	for _, tt := range []struct {
		name string
		item media.Item
		want string
	}{
		{
			name: "title only",
			item: media.Item{Title: " Rumours "},
			want: `releasegroup:"Rumours"`,
		},
		{
			name: "title and artist",
			item: media.Item{Title: "Rumours", Creator: " Fleetwood Mac "},
			want: `releasegroup:"Rumours" AND artist:"Fleetwood Mac"`,
		},
		{
			name: "Lucene special characters",
			item: media.Item{Title: "(What's the Story) Morning Glory?", Creator: "AC/DC + Oasis!"},
			want: `releasegroup:"\(What's the Story\) Morning Glory\?" AND artist:"AC\/DC \+ Oasis\!"`,
		},
		{
			name: "quotes and backslashes",
			item: media.Item{Title: `Say "Hi" \o/`, Creator: "A && B || C"},
			want: `releasegroup:"Say \"Hi\" \\o\/" AND artist:"A \&\& B \|\| C"`,
		},
		{
			name: "brackets, braces and operators",
			item: media.Item{Title: "[x] {y} ^2 ~3 *4 :5 -6"},
			want: `releasegroup:"\[x\] \{y\} \^2 \~3 \*4 \:5 \-6"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var rec recorder
			p := newTestMusicBrainz(&rec, t, 200, []byte(`{"count":0,"release-groups":[]}`))
			_, err := p.Search(context.Background(), tt.item, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
			if got := rec.last.URL.Query().Get("query"); got != tt.want {
				t.Errorf("query = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMusicBrainzJoinsArtistCredits(t *testing.T) {
	var rec recorder
	body := []byte(`{"count":1,"release-groups":[{"id":"7a4d2ac6-0000-4000-8000-000000000001","title":"Lulu",
		"primary-type":"Album","first-release-date":"2011-10-31",
		"artist-credit":[{"name":"Lou Reed","joinphrase":" & "},{"name":"Metallica"}]}]}`)
	p := newTestMusicBrainz(&rec, t, 200, body)

	got, err := p.Search(context.Background(), media.Item{Title: "Lulu", Creator: "Metallica"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Creator != "Lou Reed & Metallica" || got[0].Year != "2011" {
		t.Errorf("first = %+v, want creator %q and year 2011", got[0], "Lou Reed & Metallica")
	}
}

func TestMusicBrainzNotFound(t *testing.T) {
	t.Run("no release groups", func(t *testing.T) {
		var rec recorder
		p := newTestMusicBrainz(&rec, t, 200, []byte(`{"created":"2026-11-16T10:00:00.000Z","count":0,"offset":0,"release-groups":[]}`))
		_, err := p.Search(context.Background(), media.Item{Title: "Nonexistent Album", Creator: "Nobody"}, media.Options{})
		if !errors.Is(err, media.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("empty title makes no request", func(t *testing.T) {
		var rec recorder
		p := newTestMusicBrainz(&rec, t, 200, fixture(t, "musicbrainz_release_groups.json"))
		_, err := p.Search(context.Background(), media.Item{Title: "  ", Creator: "The Beatles"}, media.Options{})
		if !errors.Is(err, media.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		if n := rec.calls.Load(); n != 0 {
			t.Errorf("server calls = %d, want 0", n)
		}
	})
}

func TestMusicBrainzHTTPError(t *testing.T) {
	var rec recorder
	p := newTestMusicBrainz(&rec, t, 503, []byte("slow down"))
	_, err := p.Search(context.Background(), media.Item{Title: "Abbey Road"}, media.Options{})
	if err == nil || !strings.Contains(err.Error(), "503") || errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want HTTP 503", err)
	}
}

func TestAppleAlbumSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "text/javascript; charset=utf-8", fixture(t, "itunes_album_search.json"))
	p := NewAppleAlbums(srv.Client())
	p.BaseURL = srv.URL + "/search"

	if p.Name() != "apple" {
		t.Errorf("Name = %q, want apple", p.Name())
	}
	got, err := p.Search(context.Background(), media.Item{Title: "Abbey Road", Creator: "The Beatles"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	q := rec.last.URL.Query()
	for key, want := range map[string]string{
		"term": "Abbey Road The Beatles", "media": "music", "entity": "album", "country": "es", "limit": "25",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if ua := rec.last.Header.Get("User-Agent"); ua != web.APIUserAgent {
		t.Errorf("User-Agent = %q, want %q", ua, web.APIUserAgent)
	}

	// The title is the collection name; the artworkless album is skipped.
	wantTitles := []string{
		"Abbey Road (2019 Mix)",
		"Abbey Road (Remastered)",
		"Abbey Road (Remastered 2009)",
		"A Valentine's Tribute To The Beatles Abbey Road Dreamers",
		"Magical Mystery Tour",
	}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:   "Abbey Road (2019 Mix)",
		Creator: "The Beatles",
		Year:    "1969",
		ImageURLs: []string{
			"https://is1-ssl.mzstatic.com/image/thumb/Music211/v4/48/53/43/485343e3-dd6a-0034-faec-f4b6403f8108/13UMGIM63890.rgb.jpg/10000x10000bb.jpg",
		},
		Source: "apple",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
	// Album artwork is not named after an ISBN.
	for _, c := range got {
		if c.ISBN != "" {
			t.Errorf("%q ISBN = %q, want none", c.Title, c.ISBN)
		}
	}
}

func TestAppleBooksRequestUnchanged(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "apple_search.json"))
	p := NewApple(srv.Client())
	p.BaseURL = srv.URL + "/search"

	if _, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "Frank Herbert"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	const want = "country=us&limit=25&media=ebook&term=Dune+Frank+Herbert"
	if got := rec.last.URL.RawQuery; got != want {
		t.Errorf("raw query = %q, want %q", got, want)
	}
}
