package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// Taxon is the Wikidata taxon a nature name (common or scientific)
// resolves to.
type Taxon struct {
	ID    string // Wikidata Q-id
	Label string // English label, e.g. "European robin"
	Name  string // taxon name (P225), e.g. "Erithacus rubecula"
	// HasTaxonName is false when the item has no P225 and Name is the
	// label instead.
	HasTaxonName bool
	Category     string // Commons category (P373), or ""
}

// TaxonResolver resolves nature names to Wikidata taxa: a full-text search
// among taxa (P31 = Q16521), the hits ranked by label and aliases (which
// hold common and scientific names), then the best hit's claims. Results
// and not-found answers are cached per name, so providers that share a
// resolver look each name up once; concurrent calls for one name share a
// single lookup. It is safe for concurrent use.
type TaxonResolver struct {
	Client      *http.Client
	WikidataURL string

	mu       sync.Mutex
	cache    map[string]taxonResult
	inflight map[string]*taxonCall
}

type taxonResult struct {
	taxon Taxon
	err   error // nil or an ErrNotFound error
}

// taxonCall is a lookup in progress; done is closed when res is set.
type taxonCall struct {
	done chan struct{}
	res  taxonResult
}

// NewTaxonResolver returns a resolver using client (or a default one).
func NewTaxonResolver(client *http.Client) *TaxonResolver {
	if client == nil {
		client = web.NewClient()
	}
	return &TaxonResolver{
		Client:      client,
		WikidataURL: "https://www.wikidata.org/w/api.php",
	}
}

// taxonFilter restricts a Wikidata search to taxa (instance of taxon).
const taxonFilter = "haswbstatement:P31=Q16521"

// Resolve returns the taxon name names. It returns an error wrapping
// media.ErrNotFound when no taxon is plausibly named so.
func (r *TaxonResolver) Resolve(ctx context.Context, name string) (Taxon, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return Taxon{}, fmt.Errorf("%w: empty name", media.ErrNotFound)
	}
	key := match.Normalize(name)
	for {
		r.mu.Lock()
		if res, ok := r.cache[key]; ok {
			r.mu.Unlock()
			return res.taxon, res.err
		}
		call, waiting := r.inflight[key]
		if !waiting {
			call = &taxonCall{done: make(chan struct{})}
			if r.inflight == nil {
				r.inflight = map[string]*taxonCall{}
			}
			r.inflight[key] = call
		}
		r.mu.Unlock()

		if !waiting {
			return r.run(ctx, key, name, call)
		}
		select {
		case <-call.done:
		case <-ctx.Done():
			return Taxon{}, ctx.Err()
		}
		// The lookup ended for its own caller (cancelled, say), not for
		// this one: look again.
		if err := call.res.err; err != nil && ctx.Err() == nil &&
			(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			continue
		}
		return call.res.taxon, call.res.err
	}
}

// run looks name up for call and shares the result with its waiters.
func (r *TaxonResolver) run(ctx context.Context, key, name string, call *taxonCall) (Taxon, error) {
	taxon, err := r.lookup(ctx, name)
	r.mu.Lock()
	// Network and API failures are not cached: the next lookup retries.
	if err == nil || errors.Is(err, media.ErrNotFound) {
		if r.cache == nil {
			r.cache = map[string]taxonResult{}
		}
		r.cache[key] = taxonResult{taxon, err}
	}
	delete(r.inflight, key)
	call.res = taxonResult{taxon, err}
	r.mu.Unlock()
	close(call.done)
	return taxon, err
}

// taxonName is one name (label or alias) of a search hit.
type taxonName struct {
	id, label, name string
}

func (r *TaxonResolver) lookup(ctx context.Context, name string) (Taxon, error) {
	ids, err := wikidataItemSearch(ctx, r.Client, r.WikidataURL, name, taxonFilter)
	if err != nil {
		return Taxon{}, err
	}
	if len(ids) == 0 {
		return Taxon{}, fmt.Errorf("%w: no Wikidata taxon matches %q", media.ErrNotFound, name)
	}
	// Labels and aliases only: the claims of a well-known taxon weigh
	// about 150 KB, so they are fetched for the chosen hit alone.
	body, err := wikidataEntities(ctx, r.Client, r.WikidataURL, ids, "labels|aliases")
	if err != nil {
		return Taxon{}, err
	}
	var names []taxonName
	for _, id := range ids {
		e, ok := body.Entities[id]
		if !ok {
			continue
		}
		label := e.Labels["en"].Value
		if label != "" {
			names = append(names, taxonName{id, label, label})
		}
		for _, a := range e.Aliases["en"] {
			names = append(names, taxonName{id, label, a.Value})
		}
	}
	// Ties keep the search order (hits first, then label before aliases).
	ranked := match.Rank(names, media.Item{Title: name},
		func(n taxonName) string { return n.name },
		func(taxonName) string { return "" })
	if len(ranked) == 0 || !match.Plausible(media.Item{Title: name}, ranked[0].name, "") {
		return Taxon{}, fmt.Errorf("%w: no Wikidata taxon named %q", media.ErrNotFound, name)
	}
	best := ranked[0]

	claims, err := wikidataEntities(ctx, r.Client, r.WikidataURL, []string{best.id}, "claims")
	if err != nil {
		return Taxon{}, err
	}
	e := claims.Entities[best.id]
	t := Taxon{
		ID:       best.id,
		Label:    best.label,
		Name:     firstString(e.Claims["P225"]),
		Category: firstString(e.Claims["P373"]),
	}
	if t.Label == "" {
		t.Label = best.name
	}
	t.HasTaxonName = t.Name != ""
	if t.Name == "" {
		t.Name = t.Label
	}
	return t, nil
}
