package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"bujo-helper/internal/locale"
	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// OpenFoodFacts finds the front picture of a food or drink product on Open
// Food Facts. A title of 8 to 14 digits (spaces and hyphens ignored) with
// no brand is a barcode and is looked up directly; anything else is a
// product name, optionally with a brand, searched with Search-a-licious.
//
// Images are CC BY-SA 3.0, so each candidate carries a credit (product,
// contributor, licence, product page) for the sidecar. Fronts whose
// shorter side is under offMinSide pixels are skipped. It is keyless but
// rate-limited per IP (see web.DefaultThrottle) and every request sends
// the contact User-Agent.
type OpenFoodFacts struct {
	Client *http.Client
	// WorldURL serves the product API, SearchURL Search-a-licious and
	// ImagesURL the product images. Tests point them at fake servers.
	WorldURL  string
	SearchURL string
	ImagesURL string
}

// NewOpenFoodFacts returns an Open Food Facts provider using client (or a
// default one).
func NewOpenFoodFacts(client *http.Client) *OpenFoodFacts {
	if client == nil {
		client = web.NewClient()
	}
	return &OpenFoodFacts{
		Client:    client,
		WorldURL:  "https://world.openfoodfacts.org",
		SearchURL: "https://search.openfoodfacts.org",
		ImagesURL: "https://images.openfoodfacts.org",
	}
}

// Name implements media.Provider.
func (p *OpenFoodFacts) Name() string { return "openfoodfacts" }

// Label implements media.Labeler.
func (*OpenFoodFacts) Label() string { return "Open Food Facts" }

const (
	// offFields are the product fields both APIs return.
	offFields = "code,product_name,brands,lang,images"
	// offPageSize is the number of search hits read.
	offPageSize = "10"
	// offMaxCandidates caps the candidates returned.
	offMaxCandidates = 3
	// offMaxReasons caps the skipped products named in a not-found error.
	offMaxReasons = 5
	// offMinSide is the smallest accepted shorter side of a front image:
	// a 50 mm cover at about 200 dpi.
	offMinSide = 400
	// offLicense is the licence line of every credit.
	offLicense = "CC BY-SA 3.0 (https://creativecommons.org/licenses/by-sa/3.0/)"
	// offProductPage is the public product page; the credit links it
	// whatever WorldURL is.
	offProductPage = "https://world.openfoodfacts.org/product/"
)

// Search implements media.Provider. item.Title is a product name or a
// barcode; item.Creator is the brand.
func (p *OpenFoodFacts) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	lang := offLanguage(opts)
	if code, ok := offBarcode(item); ok {
		return p.lookup(ctx, code, lang)
	}
	return p.search(ctx, item, lang)
}

// lookup reads one product by barcode.
func (p *OpenFoodFacts) lookup(ctx context.Context, code, lang string) ([]media.Candidate, error) {
	var resp struct {
		Status  offScalar  `json:"status"`
		Product offProduct `json:"product"`
	}
	u := p.WorldURL + "/api/v2/product/" + code + "?" + url.Values{"fields": {offFields}}.Encode()
	err := getJSONWith(ctx, p.Client, u, web.SetOFFHeaders, &resp)
	var se *httpStatusError
	if (errors.As(err, &se) && se.Status == http.StatusNotFound) || (err == nil && resp.Status != "1") {
		return nil, fmt.Errorf("%w: no Open Food Facts product with barcode %s", media.ErrNotFound, code)
	}
	if err != nil {
		return nil, err
	}
	if resp.Product.Code == "" {
		resp.Product.Code = code
	}
	c, reason := p.candidate(resp.Product, lang)
	if reason != "" {
		return nil, offNotFound("for barcode "+code, []string{resp.Product.describe() + ": " + reason})
	}
	return []media.Candidate{c}, nil
}

// search finds products by name and brand, ranked by name match.
func (p *OpenFoodFacts) search(ctx context.Context, item media.Item, lang string) ([]media.Candidate, error) {
	v := url.Values{"q": {offQuery(item.Title, item.Creator)}, "page_size": {offPageSize}, "fields": {offFields}}
	if lang != "" {
		langs := lang
		if lang != "en" {
			langs += ",en"
		}
		v.Set("langs", langs)
	}
	var resp struct {
		Hits []offProduct `json:"hits"`
	}
	if err := getJSONWith(ctx, p.Client, p.SearchURL+"/search?"+v.Encode(), web.SetOFFHeaders, &resp); err != nil {
		return nil, err
	}
	what := strconv.Quote(strings.TrimSpace(item.Title))
	if b := strings.TrimSpace(item.Creator); b != "" {
		what += " by " + b
	}
	if len(resp.Hits) == 0 {
		return nil, fmt.Errorf("%w: no Open Food Facts product matches %s", media.ErrNotFound, what)
	}
	hits := match.Rank(resp.Hits, item,
		func(h offProduct) string { return h.ProductName },
		func(h offProduct) string { return strings.Join(h.Brands, ", ") })
	var out []media.Candidate
	var skipped []string
	for _, h := range hits {
		c, reason := p.candidate(h, lang)
		if reason != "" {
			skipped = append(skipped, h.describe()+": "+reason)
			continue
		}
		if out = append(out, c); len(out) == offMaxCandidates {
			break
		}
	}
	if len(out) == 0 {
		return nil, offNotFound("for "+what, skipped)
	}
	return out, nil
}

var offDigits = regexp.MustCompile(`^[0-9]+$`)

// candidate turns a product into a candidate, or returns why it is
// skipped.
func (p *OpenFoodFacts) candidate(prod offProduct, lang string) (media.Candidate, string) {
	if !offDigits.MatchString(prod.Code) {
		return media.Candidate{}, "invalid barcode"
	}
	key, img, reason := prod.front(lang)
	if reason != "" {
		return media.Candidate{}, reason
	}
	title := offText(prod.ProductName)
	if title == "" {
		title = prod.Code
	}
	brand := offText(prod.Brands.first())
	return media.Candidate{
		Title:     title,
		Creator:   brand,
		ImageURLs: []string{offImageURL(p.ImagesURL, prod.Code, key, string(img.Rev))},
		Source:    p.Name(),
		Credit:    offCredit(title, brand, prod.uploader(img.ImgID), prod.Code),
	}, ""
}

// offBarcode returns the barcode of item: its title without spaces and
// hyphens when that is 8 to 14 ASCII digits and no brand is given.
func offBarcode(item media.Item) (string, bool) {
	if strings.TrimSpace(item.Creator) != "" {
		return "", false
	}
	code := strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(item.Title))
	if len(code) < 8 || len(code) > 14 || !offDigits.MatchString(code) {
		return "", false
	}
	return code, true
}

// offImageURL is the full-size URL of image key (e.g. "front_en") at
// revision rev of a product. Codes of up to 8 digits are their own folder;
// longer ones are left-padded to 13 digits and split 3/3/3/rest.
func offImageURL(base, code, key, rev string) string {
	folder := code
	if len(code) > 8 {
		if len(code) < 13 {
			code = strings.Repeat("0", 13-len(code)) + code
		}
		folder = code[:3] + "/" + code[3:6] + "/" + code[6:9] + "/" + code[9:]
	}
	return base + "/images/products/" + folder + "/" + key + "." + rev + ".full.jpg"
}

// offQuery is the Search-a-licious (Lucene) query for a name and brand:
// the name with special characters escaped and the operator words AND,
// OR and NOT lowercased (Lucene only reads them as operators in capitals,
// and the search ignores case), then a brands phrase.
func offQuery(name, brand string) string {
	var b strings.Builder
	for i, word := range strings.Fields(name) {
		if i > 0 {
			b.WriteByte(' ')
		}
		switch word {
		case "AND", "OR", "NOT":
			word = strings.ToLower(word)
		}
		for _, r := range word {
			if strings.ContainsRune(`+-&|!(){}[]^"~*?:\/`, r) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
	}
	if brand = strings.TrimSpace(brand); brand != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(`brands:"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(brand) + `"`)
	}
	return b.String()
}

// offMARC maps the MARC language codes of media.Options to the ISO 639-1
// codes Open Food Facts uses.
var offMARC = map[string]string{
	"eng": "en", "spa": "es", "fre": "fr", "fra": "fr", "ger": "de", "deu": "de",
	"ita": "it", "por": "pt", "cat": "ca", "dut": "nl", "nld": "nl",
}

var offISO = regexp.MustCompile(`^[a-z]{2}$`)

// offLanguage returns the ISO 639-1 language preferred by opts: its
// Language (MARC or ISO 639-1), else the language of its Country; "" when
// unknown.
func offLanguage(opts media.Options) string {
	lang := strings.ToLower(strings.TrimSpace(opts.Language))
	if lang == "" {
		lang = locale.For(opts.Country).Language
	}
	if offISO.MatchString(lang) {
		return lang
	}
	return offMARC[lang]
}

// offText folds s onto one line and caps its length, for credit fields.
func offText(s string) string { return capField(strings.Join(strings.Fields(s), " ")) }

// offCredit builds the credit text of a product image: the product, the
// contributor (when known), the source, the licence and the product page.
func offCredit(title, brand, uploader, code string) string {
	head := "Title: " + offText(title)
	if b := offText(brand); b != "" {
		head += " (" + b + ")"
	}
	lines := []string{head}
	if u := offText(uploader); u != "" {
		lines = append(lines, "Contributor: "+u)
	}
	lines = append(lines, "Source: Open Food Facts", "License: "+offLicense, "Product page: "+offProductPage+code)
	return strings.Join(lines, "\n")
}

// offNotFound is the error when no product gave a usable image, naming
// the first products skipped and why.
func offNotFound(what string, skipped []string) error {
	msg := "no usable Open Food Facts product image " + what
	if len(skipped) > 0 {
		shown := skipped
		if len(shown) > offMaxReasons {
			shown = shown[:offMaxReasons]
		}
		msg += "; skipped " + strings.Join(shown, "; ")
		if more := len(skipped) - len(shown); more > 0 {
			msg += fmt.Sprintf("; and %d more", more)
		}
	}
	return fmt.Errorf("%w: %s", media.ErrNotFound, msg)
}

// offProduct is a product of either API. Images holds both the selected
// images ("front_en") and the uploads they come from ("123"); entries are
// decoded one by one so a malformed one only loses that image.
type offProduct struct {
	Code        string                     `json:"code"`
	ProductName string                     `json:"product_name"`
	Brands      offBrands                  `json:"brands"`
	Lang        string                     `json:"lang"`
	Images      map[string]json.RawMessage `json:"images"`
}

// offImage is a selected image or an upload. The API returns ids,
// revisions and sizes as numbers or strings.
type offImage struct {
	ImgID offScalar `json:"imgid"`
	Rev   offScalar `json:"rev"`
	Sizes struct {
		Full struct {
			W offScalar `json:"w"`
			H offScalar `json:"h"`
		} `json:"full"`
	} `json:"sizes"`
	Uploader string `json:"uploader"`
}

// image decodes one entry of the images map; a missing or malformed one
// is zero.
func (p offProduct) image(key string) offImage {
	var img offImage
	if raw, ok := p.Images[key]; ok {
		if json.Unmarshal(raw, &img) != nil {
			return offImage{}
		}
	}
	return img
}

// uploader is the contributor who uploaded image imgid, if known.
func (p offProduct) uploader(imgid offScalar) string {
	if imgid == "" {
		return ""
	}
	return p.image(string(imgid)).Uploader
}

// describe names a product in skip reasons: "<name> (<code>)".
func (p offProduct) describe() string {
	if name := offText(p.ProductName); name != "" {
		return name + " (" + p.Code + ")"
	}
	return p.Code
}

var offFrontKey = regexp.MustCompile(`^front_[a-z]+$`)

// front picks the front image to use: front_<lang>, front_<product
// language>, front_en, then any other front_* in key order; the first one
// that is large enough wins. Otherwise it returns why the product is
// skipped (the reason of the first front tried).
func (p offProduct) front(lang string) (string, offImage, string) {
	var keys []string
	for _, l := range []string{lang, strings.ToLower(p.Lang), "en"} {
		if k := "front_" + l; l != "" && !slices.Contains(keys, k) && offFrontKey.MatchString(k) && p.Images[k] != nil {
			keys = append(keys, k)
		}
	}
	var rest []string
	for k := range p.Images {
		if offFrontKey.MatchString(k) && !slices.Contains(keys, k) {
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	keys = append(keys, rest...)
	if len(keys) == 0 {
		return "", offImage{}, "no front image"
	}
	first := ""
	for _, k := range keys {
		img := p.image(k)
		reason := img.unusable()
		if reason == "" {
			return k, img, ""
		}
		if first == "" {
			first = reason
		}
	}
	return "", offImage{}, first
}

// unusable returns why a front image cannot be used, or "".
func (img offImage) unusable() string {
	w, errW := strconv.Atoi(string(img.Sizes.Full.W))
	h, errH := strconv.Atoi(string(img.Sizes.Full.H))
	switch {
	case errW != nil || errH != nil || w <= 0 || h <= 0:
		return "front image size unknown"
	case min(w, h) < offMinSide:
		return fmt.Sprintf("front image too small (%d×%d px)", w, h)
	case !offDigits.MatchString(string(img.Rev)):
		return "front image revision unknown"
	}
	return ""
}

// offScalar is a JSON string or number, kept as text; null is "".
type offScalar string

// UnmarshalJSON implements json.Unmarshaler.
func (s *offScalar) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = offScalar(strings.TrimSpace(v))
		return nil
	}
	if string(b) == "null" {
		*s = ""
		return nil
	}
	*s = offScalar(b)
	return nil
}

// offBrands are a product's brands: the product API returns them as a
// comma-separated string, Search-a-licious as an array.
type offBrands []string

// UnmarshalJSON implements json.Unmarshaler.
func (bs *offBrands) UnmarshalJSON(b []byte) error {
	var list []string
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		list = strings.Split(s, ",")
	} else if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	*bs = nil
	for _, v := range list {
		if v = strings.TrimSpace(v); v != "" {
			*bs = append(*bs, v)
		}
	}
	return nil
}

// first is the first brand, or "".
func (bs offBrands) first() string {
	if len(bs) == 0 {
		return ""
	}
	return bs[0]
}
