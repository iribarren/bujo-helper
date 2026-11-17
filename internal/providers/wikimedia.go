package providers

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// Wikimedia finds places (landmarks, cities) on Wikidata and downloads their
// image (P18) from Wikimedia Commons. A title such as "Flag of Japan" or
// "Japan flag" switches to flag mode: the country's flag (P41) is used and
// the country column is ignored. It is keyless and makes at most four
// requests per item: search, entities, country labels and imageinfo.
//
// Wikidata is CC0, but every Commons file keeps its own licence, so each
// candidate carries a credit (author, licence, file page) for the sidecar.
type Wikimedia struct {
	Client      *http.Client
	WikidataURL string
	CommonsURL  string
}

// NewWikimedia returns a Wikidata → Commons provider using client (or a
// default one).
func NewWikimedia(client *http.Client) *Wikimedia {
	if client == nil {
		client = web.NewClient()
	}
	return &Wikimedia{
		Client:      client,
		WikidataURL: "https://www.wikidata.org/w/api.php",
		CommonsURL:  "https://commons.wikimedia.org/w/api.php",
	}
}

// Name implements media.Provider.
func (p *Wikimedia) Name() string { return "wikimedia" }

// wikimediaMaxImages caps the hits whose Commons image info is fetched,
// and so the candidates of a place search.
const wikimediaMaxImages = 3

// Commons thumbnail widths. Only standard widths are served (see
// https://www.mediawiki.org/wiki/Common_thumbnail_sizes); other widths are
// rejected, so the 1280 URL is derived from the 1920 one Commons returns.
const (
	commonsThumbWidth    = "1920"
	commonsThumbSegment  = "/1920px-"
	commonsSmallerThumb  = "/1280px-"
	commonsMetadataNames = "ObjectName|Artist|Credit|Attribution|LicenseShortName|LicenseUrl|AttributionRequired|Copyrighted"
)

// wikimediaClaim is a statement: its main value (a string for files, an
// object with "id" for items) and its rank.
type wikimediaClaim struct {
	Mainsnak struct {
		Datavalue struct {
			Value any `json:"value"`
		} `json:"datavalue"`
	} `json:"mainsnak"`
	Rank string `json:"rank"`
}

type wikimediaText struct {
	Value string `json:"value"`
}

type wikimediaEntitiesResponse struct {
	Error    *wikidataError `json:"error"`
	Entities map[string]struct {
		Labels       map[string]wikimediaText    `json:"labels"`
		Descriptions map[string]wikimediaText    `json:"descriptions"`
		Claims       map[string][]wikimediaClaim `json:"claims"`
	} `json:"entities"`
}

// wikimediaPlace is a Wikidata hit with an image file.
type wikimediaPlace struct {
	id, label, description string
	countries              []string // P17 Q-ids
	countryLabels          []string // their English labels, when looked up
	file                   string   // Commons file name, without "File:"
}

// Search implements media.Provider. item.Creator is the country, if known:
// hits in it rank first.
func (p *Wikimedia) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	if c, ok := flagCountry(title); ok {
		return p.searchFlag(ctx, c)
	}
	return p.searchPlace(ctx, title, strings.TrimSpace(item.Creator))
}

func (p *Wikimedia) searchPlace(ctx context.Context, title, country string) ([]media.Candidate, error) {
	places, err := p.places(ctx, title, "P18")
	if err != nil {
		return nil, err
	}
	if len(places) == 0 {
		return nil, fmt.Errorf("%w: no Wikidata places with an image", media.ErrNotFound)
	}
	ranked := rankPlaces(places, title)
	if country != "" {
		if err := p.countryLabels(ctx, ranked); err != nil {
			return nil, err
		}
		ranked = preferCountry(ranked, normalizeCountry(country))
	}
	return p.candidates(ctx, distinctFiles(ranked, wikimediaMaxImages), func(pl wikimediaPlace) (string, string) {
		var c string
		if len(pl.countryLabels) > 0 {
			c = pl.countryLabels[0]
		}
		return pl.label, c
	})
}

// searchFlag uses the flag (P41) of the hit whose label best matches
// country.
func (p *Wikimedia) searchFlag(ctx context.Context, country string) ([]media.Candidate, error) {
	places, err := p.places(ctx, country, "P41")
	if err != nil {
		return nil, err
	}
	if len(places) == 0 {
		return nil, fmt.Errorf("%w: no Wikidata countries with a flag", media.ErrNotFound)
	}
	// Aliases such as "USA" rank by the country's Wikidata label.
	query := normalizeCountry(country)
	best := rankPlaces(places, query)[:1]
	// The search returns any entity with a flag, so the best one must
	// still be a plausible name match (exact or prefix).
	if !match.Plausible(media.Item{Title: query}, best[0].label, "") {
		return nil, fmt.Errorf("%w: no Wikidata country named %q with a flag", media.ErrNotFound, country)
	}
	return p.candidates(ctx, best, func(pl wikimediaPlace) (string, string) {
		return "Flag of " + pl.label, ""
	})
}

// places searches Wikidata for q among entities with property prop, and
// returns, in search order, those whose prop holds a usable file.
func (p *Wikimedia) places(ctx context.Context, q, prop string) ([]wikimediaPlace, error) {
	ids, err := p.search(ctx, q+" haswbstatement:"+prop)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	body, err := p.entities(ctx, ids, "labels|descriptions|claims")
	if err != nil {
		return nil, err
	}
	var places []wikimediaPlace
	for _, id := range ids {
		e, ok := body.Entities[id]
		if !ok {
			continue
		}
		file := firstFile(e.Claims[prop])
		if file == "" {
			continue
		}
		pl := wikimediaPlace{
			id:          id,
			label:       e.Labels["en"].Value,
			description: e.Descriptions["en"].Value,
			file:        file,
		}
		if pl.label == "" {
			pl.label = q
		}
		for _, c := range e.Claims["P17"] {
			if v, ok := c.Mainsnak.Datavalue.Value.(map[string]any); ok && c.Rank != "deprecated" {
				if qid, _ := v["id"].(string); qid != "" {
					pl.countries = append(pl.countries, qid)
				}
			}
		}
		places = append(places, pl)
	}
	return places, nil
}

// firstFile returns the file of the first preferred statement, else of
// the first normal one. Deprecated statements are never used.
func firstFile(claims []wikimediaClaim) string {
	var normal string
	for _, c := range claims {
		v, ok := c.Mainsnak.Datavalue.Value.(string)
		if !ok || v == "" {
			continue
		}
		switch c.Rank {
		case "preferred":
			return v
		case "deprecated":
		default:
			if normal == "" {
				normal = v
			}
		}
	}
	return normal
}

// search returns the Q-ids the Wikidata full-text search finds for q.
func (p *Wikimedia) search(ctx context.Context, q string) ([]string, error) {
	v := url.Values{
		"action":      {"query"},
		"list":        {"search"},
		"srsearch":    {q},
		"srnamespace": {"0"},
		"srlimit":     {strconv.Itoa(wikidataSearchLimit)},
		"srprop":      {""},
		"format":      {"json"},
	}
	var body wikidataSearchResponse
	if err := getJSONWith(ctx, p.Client, p.WikidataURL+"?"+v.Encode(), web.SetWikimediaHeaders, &body); err != nil {
		return nil, err
	}
	if err := body.Error.err(); err != nil {
		return nil, err
	}
	var ids []string
	for _, s := range body.Query.Search {
		if strings.HasPrefix(s.Title, "Q") {
			ids = append(ids, s.Title)
		}
	}
	return ids, nil
}

func (p *Wikimedia) entities(ctx context.Context, ids []string, props string) (*wikimediaEntitiesResponse, error) {
	v := url.Values{
		"action":    {"wbgetentities"},
		"ids":       {strings.Join(ids, "|")},
		"props":     {props},
		"languages": {"en"},
		// Some entities only have a language-neutral ("mul") label.
		"languagefallback": {"1"},
		"format":           {"json"},
	}
	var body wikimediaEntitiesResponse
	if err := getJSONWith(ctx, p.Client, p.WikidataURL+"?"+v.Encode(), web.SetWikimediaHeaders, &body); err != nil {
		return nil, err
	}
	if err := body.Error.err(); err != nil {
		return nil, err
	}
	return &body, nil
}

// countryLabels looks up the English labels of the places' countries in
// one request.
func (p *Wikimedia) countryLabels(ctx context.Context, places []wikimediaPlace) error {
	var ids []string
	seen := map[string]bool{}
	for _, pl := range places {
		for _, id := range pl.countries {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	body, err := p.entities(ctx, ids, "labels")
	if err != nil {
		return err
	}
	for i := range places {
		for _, id := range places[i].countries {
			if l := body.Entities[id].Labels["en"].Value; l != "" {
				places[i].countryLabels = append(places[i].countryLabels, l)
			}
		}
	}
	return nil
}

// rankPlaces ranks places by label against query (see match.Rank); ties
// keep the search order.
func rankPlaces(places []wikimediaPlace, query string) []wikimediaPlace {
	return match.Rank(places, media.Item{Title: query},
		func(pl wikimediaPlace) string { return pl.label },
		func(wikimediaPlace) string { return "" })
}

// preferCountry moves the places in country (normalized) first, keeping
// the order within both groups.
func preferCountry(places []wikimediaPlace, country string) []wikimediaPlace {
	var in, out []wikimediaPlace
	for _, pl := range places {
		if placeInCountry(country, pl.countryLabels, pl.description) {
			in = append(in, pl)
		} else {
			out = append(out, pl)
		}
	}
	return append(in, out...)
}

// distinctFiles returns the first n places with different files.
func distinctFiles(places []wikimediaPlace, n int) []wikimediaPlace {
	var out []wikimediaPlace
	seen := map[string]bool{}
	for _, pl := range places {
		if len(out) == n {
			break
		}
		if !seen[pl.file] {
			seen[pl.file] = true
			out = append(out, pl)
		}
	}
	return out
}

// countryAliases maps a few common short names (normalized) to the
// Wikidata label of the country.
var countryAliases = map[string]string{
	"usa":                      "united states",
	"us":                       "united states",
	"u s":                      "united states",
	"u s a":                    "united states",
	"united states of america": "united states",
	"america":                  "united states",
	"uk":                       "united kingdom",
	"u k":                      "united kingdom",
	"great britain":            "united kingdom",
	"britain":                  "united kingdom",
}

// normalizeCountry folds a country name for comparison and resolves the
// aliases.
func normalizeCountry(s string) string {
	n := match.Normalize(s)
	if a, ok := countryAliases[n]; ok {
		return a
	}
	return n
}

// placeInCountry reports whether one of a place's country labels is
// country (normalized), or its description names it as whole words.
func placeInCountry(country string, labels []string, description string) bool {
	if country == "" {
		return false
	}
	for _, l := range labels {
		if normalizeCountry(l) == country {
			return true
		}
	}
	return strings.Contains(" "+match.Normalize(description)+" ", " "+country+" ")
}

var (
	flagOfPattern = regexp.MustCompile(`(?i)^flag\s+of\s+(.+)$`)
	flagPattern   = regexp.MustCompile(`(?i)^(.+?)\s+flag$`)
	leadingThe    = regexp.MustCompile(`(?i)^the\s+`)
)

// flagCountry returns X for a title "Flag of X" or "X flag" (any case).
// A leading "the" is dropped, as in "Flag of the United Kingdom".
func flagCountry(title string) (string, bool) {
	t := strings.Join(strings.Fields(title), " ")
	var c string
	if m := flagOfPattern.FindStringSubmatch(t); m != nil {
		c = m[1]
	} else if m := flagPattern.FindStringSubmatch(t); m != nil {
		c = m[1]
	}
	c = strings.TrimSpace(leadingThe.ReplaceAllString(c, ""))
	return c, c != ""
}

// --- Commons ---

type commonsMetadataValue struct {
	// Value is HTML; with the metadata filter used, always a string.
	Value any `json:"value"`
}

type commonsImage struct {
	URL            string                          `json:"url"`
	ThumbURL       string                          `json:"thumburl"`
	DescriptionURL string                          `json:"descriptionurl"`
	ExtMetadata    map[string]commonsMetadataValue `json:"extmetadata"`
}

// metadata returns the extmetadata field name as plain text.
func (img commonsImage) metadata(name string) string {
	switch v := img.ExtMetadata[name].Value.(type) {
	case string:
		return htmlText(v)
	case nil:
		return ""
	default:
		return htmlText(fmt.Sprint(v))
	}
}

type commonsImageInfoResponse struct {
	Error *wikidataError `json:"error"`
	Query struct {
		Normalized []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"normalized"`
		Pages map[string]struct {
			Title     string         `json:"title"`
			ImageInfo []commonsImage `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

// candidates fetches the Commons image info of the places' files in one
// request and returns, in places order, a candidate for each file Commons
// has. names gives a place's candidate title and creator.
func (p *Wikimedia) candidates(ctx context.Context, places []wikimediaPlace, names func(wikimediaPlace) (title, creator string)) ([]media.Candidate, error) {
	titles := make([]string, len(places))
	for i, pl := range places {
		titles[i] = "File:" + pl.file
	}
	images, err := p.imageInfo(ctx, titles)
	if err != nil {
		return nil, err
	}
	var out []media.Candidate
	for i, pl := range places {
		img, ok := images[titles[i]]
		if !ok {
			continue
		}
		urls := commonsImageURLs(img)
		if len(urls) == 0 {
			continue
		}
		title, creator := names(pl)
		out = append(out, media.Candidate{
			Title:     title,
			Creator:   creator,
			ImageURLs: urls,
			Source:    p.Name(),
			Credit:    commonsCredit(img, title, pl.id),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no Commons image for %d Wikidata matches", media.ErrNotFound, len(places))
	}
	return out, nil
}

// imageInfo returns the image info of the files titles (as requested,
// "File:…"), leaving out missing files.
func (p *Wikimedia) imageInfo(ctx context.Context, titles []string) (map[string]commonsImage, error) {
	v := url.Values{
		"action":              {"query"},
		"prop":                {"imageinfo"},
		"titles":              {strings.Join(titles, "|")},
		"iiprop":              {"url|size|extmetadata"},
		"iiurlwidth":          {commonsThumbWidth},
		"iiextmetadatafilter": {commonsMetadataNames},
		"format":              {"json"},
	}
	var body commonsImageInfoResponse
	if err := getJSONWith(ctx, p.Client, p.CommonsURL+"?"+v.Encode(), web.SetWikimediaHeaders, &body); err != nil {
		return nil, err
	}
	if err := body.Error.err(); err != nil {
		return nil, err
	}
	byTitle := map[string]commonsImage{}
	for _, page := range body.Query.Pages {
		if len(page.ImageInfo) > 0 {
			byTitle[page.Title] = page.ImageInfo[0]
		}
	}
	// Commons may normalize the requested titles (e.g. "_" to " ").
	normalized := map[string]string{}
	for _, n := range body.Query.Normalized {
		normalized[n.From] = n.To
	}
	out := map[string]commonsImage{}
	for _, t := range titles {
		key := t
		if n, ok := normalized[t]; ok {
			key = n
		}
		if img, ok := byTitle[key]; ok {
			out[t] = img
		}
	}
	return out, nil
}

// commonsImageURLs returns the 1920 thumbnail and the 1280 one, best
// first. Files narrower than 1920 come back unscaled: only that URL.
func commonsImageURLs(img commonsImage) []string {
	if img.ThumbURL == "" {
		if img.URL == "" {
			return nil
		}
		return []string{img.URL}
	}
	urls := []string{img.ThumbURL}
	if strings.Contains(img.ThumbURL, commonsThumbSegment) {
		urls = append(urls, strings.Replace(img.ThumbURL, commonsThumbSegment, commonsSmallerThumb, 1))
	}
	return urls
}

// commonsFieldLimit caps each credit field, in runes: some Commons
// descriptions hold whole tables or galleries.
const commonsFieldLimit = 300

// commonsCredit builds the credit text of a Commons file: title, author,
// source, licence, file page and Wikidata item. label is the title when
// the file has no object name.
func commonsCredit(img commonsImage, label, qid string) string {
	title := img.metadata("ObjectName")
	if title == "" {
		title = label
	}
	lines := []string{"Title: " + capField(title)}
	// Attribution, when set, is the credit line the author asks for.
	author := img.metadata("Attribution")
	if author == "" {
		author = img.metadata("Artist")
	}
	if author != "" {
		lines = append(lines, "Author: "+capField(author))
	}
	if source := img.metadata("Credit"); source != "" && source != author {
		lines = append(lines, "Source: "+capField(source))
	}
	lines = append(lines, "License: "+commonsLicense(img.metadata("LicenseShortName"), img.metadata("LicenseUrl"), img.metadata("Copyrighted")))
	if img.DescriptionURL != "" {
		lines = append(lines, "File page: "+img.DescriptionURL)
	}
	lines = append(lines, "Wikidata item: https://www.wikidata.org/wiki/"+qid)
	return strings.Join(lines, "\n")
}

// commonsLicense describes a file's licence: its short name and URL,
// marked as public domain when Commons says it is not copyrighted.
func commonsLicense(short, licenseURL, copyrighted string) string {
	public := strings.EqualFold(copyrighted, "false")
	switch {
	case short == "" && public:
		return "Public domain"
	case short == "":
		return "unknown, see the file page"
	}
	s := short
	if public && !strings.Contains(strings.ToLower(short), "public domain") {
		s += " (public domain)"
	}
	if licenseURL != "" {
		s += ", " + licenseURL
	}
	return s
}

// capField shortens s to commonsFieldLimit runes, ending it with "…".
func capField(s string) string {
	r := []rune(s)
	if len(r) <= commonsFieldLimit {
		return s
	}
	return strings.TrimSpace(string(r[:commonsFieldLimit])) + "…"
}

var (
	// htmlBlockTag matches tags that separate words (line breaks, blocks,
	// list items, table cells).
	htmlBlockTag = regexp.MustCompile(`(?i)<\s*/?\s*(br|p|div|li|ul|ol|dl|dt|dd|td|th|tr|table|tbody|thead|h[1-6])\b[^>]*>`)
	htmlTag      = regexp.MustCompile(`<[^>]*>`)
)

// htmlText turns a Commons metadata HTML fragment into one line of plain
// text: tags are dropped (block tags become spaces), entities decoded and
// whitespace collapsed.
func htmlText(s string) string {
	s = htmlBlockTag.ReplaceAllString(s, " ")
	s = htmlTag.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}
