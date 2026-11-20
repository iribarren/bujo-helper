package providers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// CommonsNature finds a vintage natural-history illustration (a plate, not
// a photo) of a plant or animal on Wikimedia Commons. The name (common or
// scientific) is resolved to a Wikidata taxon first; then Commons files
// are searched, in order, until one search gives an openly licensed image:
//
//  1. files that depict the taxon and are marked public domain;
//  2. the "<Commons category> - botanical illustrations" category (plants);
//  3. files whose text holds the taxon name and "plate".
//
// Only public domain, CC0, CC BY and CC BY-SA files are used; each
// candidate carries a credit (author, date, source, licence, file page)
// for the sidecar. It is keyless: at most three Wikidata requests per name
// (cached) and two Commons requests per search.
type CommonsNature struct {
	Client     *http.Client
	CommonsURL string
	// Taxa resolves names; providers may share one to look each name up
	// once.
	Taxa *TaxonResolver
}

// NewCommonsNature returns a Commons nature provider using client (or a
// default one) and taxa (or a new resolver).
func NewCommonsNature(client *http.Client, taxa *TaxonResolver) *CommonsNature {
	if client == nil {
		client = web.NewClient()
	}
	if taxa == nil {
		taxa = NewTaxonResolver(client)
	}
	return &CommonsNature{
		Client:     client,
		CommonsURL: "https://commons.wikimedia.org/w/api.php",
		Taxa:       taxa,
	}
}

// Name implements media.Provider.
func (p *CommonsNature) Name() string { return "commons-nature" }

// Label implements media.Labeler.
func (*CommonsNature) Label() string { return "Wikimedia Commons" }

const (
	// natureMaxImages caps the files of a search whose image info is
	// fetched.
	natureMaxImages = 5
	// natureMaxCandidates caps the candidates returned.
	natureMaxCandidates = 3
	// natureMaxReasons caps the skipped files named in a not-found error.
	natureMaxReasons = 5
	// natureMetadataNames adds the date to the credit fields.
	natureMetadataNames = commonsMetadataNames + "|DateTimeOriginal"
	// bhlCreditLine ends the credit of an image from the Biodiversity
	// Heritage Library, as BHL asks.
	bhlCreditLine = "Image from the Biodiversity Heritage Library"
)

// Search implements media.Provider. item.Title is the name; item.Creator
// is not used.
func (p *CommonsNature) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	taxon, err := p.Taxa.Resolve(ctx, item.Title)
	if err != nil {
		return nil, err
	}
	var skipped []string
	seen := map[string]bool{}
	for _, q := range natureSearches(taxon) {
		titles, err := mediawikiSearch(ctx, p.Client, p.CommonsURL, q, "6")
		if err != nil {
			return nil, err
		}
		if len(titles) == 0 {
			continue
		}
		titles = platesFirst(titles)
		if len(titles) > natureMaxImages {
			titles = titles[:natureMaxImages]
		}
		images, err := commonsImageInfo(ctx, p.Client, p.CommonsURL, titles, natureMetadataNames)
		if err != nil {
			return nil, err
		}
		var first, rest []media.Candidate
		for _, title := range titles {
			img, ok := images[title]
			if !ok {
				continue
			}
			reason := openLicense(img.visibleMetadata("LicenseShortName"), img.visibleMetadata("LicenseUrl"), img.visibleMetadata("Copyrighted"))
			if reason != "" {
				if !seen[title] {
					seen[title] = true
					skipped = append(skipped, strings.TrimPrefix(title, "File:")+": "+reason)
				}
				continue
			}
			urls := commonsImageURLs(img)
			if len(urls) == 0 {
				continue
			}
			c := media.Candidate{
				Title:     taxon.Label,
				Creator:   taxonCreator(taxon),
				ImageURLs: urls,
				Source:    p.Name(),
				Credit:    natureCredit(img, taxon),
			}
			// Plates and BHL scans first; the rest keep the search order.
			if isPlate(title) || fromBHL(img) {
				first = append(first, c)
			} else {
				rest = append(rest, c)
			}
		}
		if out := append(first, rest...); len(out) > 0 {
			if len(out) > natureMaxCandidates {
				out = out[:natureMaxCandidates]
			}
			return out, nil
		}
	}
	return nil, natureNotFound(taxon, skipped)
}

// natureSearches returns the Commons searches for taxon, in order.
func natureSearches(t Taxon) []string {
	unquote := strings.NewReplacer(`"`, "").Replace
	qs := []string{"haswbstatement:P180=" + t.ID + " haswbstatement:P6216=Q19652 filetype:bitmap"}
	if t.Category != "" {
		qs = append(qs, `incategory:"`+unquote(t.Category)+` - botanical illustrations"`)
	}
	return append(qs, `"`+unquote(t.Name)+`" plate filetype:bitmap`)
}

// taxonCreator is the candidate creator of taxon: its scientific name, when
// it is not the label already.
func taxonCreator(t Taxon) string {
	if match.Normalize(t.Name) == match.Normalize(t.Label) {
		return ""
	}
	return t.Name
}

var platePattern = regexp.MustCompile(`(?i)\bplates?\b`)

// isPlate reports whether a file title names a plate.
func isPlate(title string) bool { return platePattern.MatchString(title) }

// platesFirst moves the titles that name a plate first, keeping the order
// within both groups.
func platesFirst(titles []string) []string {
	var plates, rest []string
	for _, t := range titles {
		if isPlate(t) {
			plates = append(plates, t)
		} else {
			rest = append(rest, t)
		}
	}
	return append(plates, rest...)
}

// fromBHL reports whether a file credits the Biodiversity Heritage Library
// (its Flickr account or its site) as its source.
func fromBHL(img commonsImage) bool {
	credit := strings.ToLower(img.rawMetadata("Credit"))
	return strings.Contains(credit, "flickr.com/photos/biodivlibrary") || strings.Contains(credit, "biodiversitylibrary.org")
}

// natureCredit builds the credit text of a Commons illustration of taxon:
// title, author, date, source, licence, file page, Wikidata taxon and, for
// BHL scans, the line BHL asks for.
func natureCredit(img commonsImage, taxon Taxon) string {
	lines := commonsCreditLines(img, taxon.Label, img.visibleMetadata, true)
	lines = append(lines, "Wikidata taxon: https://www.wikidata.org/wiki/"+taxon.ID)
	if fromBHL(img) {
		lines = append(lines, bhlCreditLine)
	}
	return strings.Join(lines, "\n")
}

// openLicense returns "" when a file's licence allows printing it with a
// credit (public domain, CC0, CC BY, CC BY-SA), else why it is skipped.
// short, licenseURL and copyrighted are the Commons LicenseShortName,
// LicenseUrl and Copyrighted fields.
func openLicense(short, licenseURL, copyrighted string) string {
	lower := strings.ToLower(short)
	tokens := map[string]bool{}
	for _, t := range strings.FieldsFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		tokens[t] = true
	}
	u := strings.ToLower(licenseURL)
	name := short
	if name == "" {
		name = licenseURL
	}
	switch {
	case tokens["nc"] || strings.Contains(u, "-nc"):
		return "non-commercial licence (" + name + ")"
	case tokens["nd"] || strings.Contains(u, "-nd"):
		return "no-derivatives licence (" + name + ")"
	case strings.EqualFold(copyrighted, "false"),
		strings.Contains(lower, "public domain"), tokens["pd"], tokens["cc0"],
		strings.Contains(u, "/publicdomain/"):
		return ""
	case tokens["cc"] && tokens["by"],
		strings.Contains(u, "creativecommons.org/licenses/by/"),
		strings.Contains(u, "creativecommons.org/licenses/by-sa/"):
		return ""
	case name == "":
		return "no licence information"
	}
	return "licence not accepted (" + name + ")"
}

// natureNotFound is the error when no search gave an open image, naming
// the first files skipped for their licence.
func natureNotFound(t Taxon, skipped []string) error {
	msg := fmt.Sprintf("no openly licensed Commons illustration of %s (%s)", t.Label, t.ID)
	if len(skipped) > 0 {
		shown := skipped
		if len(shown) > natureMaxReasons {
			shown = shown[:natureMaxReasons]
		}
		msg += "; skipped " + strings.Join(shown, "; ")
		if more := len(skipped) - len(shown); more > 0 {
			msg += fmt.Sprintf("; and %d more", more)
		}
	}
	return fmt.Errorf("%w: %s", media.ErrNotFound, msg)
}
