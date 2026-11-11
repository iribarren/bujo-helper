// Package media holds the domain types shared by providers, the downloader
// and the user interfaces. It has no dependencies on the rest of the app.
package media

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound reports that a provider found no usable match for an item.
var ErrNotFound = errors.New("not found")

// Item is one thing to find a cover for, e.g. a book title and its author.
type Item struct {
	Title   string
	Creator string
}

// Candidate is one search hit returned by a provider.
type Candidate struct {
	Title   string
	Creator string
	Year    string
	// ImageURLs are cover image URLs ordered best-first.
	ImageURLs []string
	// Source is the name of the provider that returned the candidate.
	Source string
	// ISBN is the ISBN-13 of the edition the cover belongs to, or "" when
	// the provider does not know it.
	ISBN string
}

// Options tunes a search.
type Options struct {
	// Country is a lowercase ISO country code used by store-backed providers.
	Country string
	// Language is an Open Library (MARC) language code such as "spa". When
	// set, providers that can filter by language look for editions in it.
	Language string
}

// Provider searches one source of cover images.
type Provider interface {
	Name() string
	// Search returns candidates ranked best-first. It returns an error
	// wrapping ErrNotFound when there are no usable results.
	Search(ctx context.Context, item Item, opts Options) ([]Candidate, error)
}

// MediaType describes a kind of media and the providers that serve it,
// tried in order.
type MediaType struct {
	ID           string
	Label        string
	CreatorLabel string
	Providers    []Provider
}

// Registry is an ordered collection of media types.
type Registry struct {
	types []MediaType
	index map[string]int
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{index: map[string]int{}}
}

// Register adds a media type. It fails if the ID is empty or already used.
func (r *Registry) Register(t MediaType) error {
	if t.ID == "" {
		return errors.New("media type ID is empty")
	}
	if _, ok := r.index[t.ID]; ok {
		return fmt.Errorf("media type %q already registered", t.ID)
	}
	r.index[t.ID] = len(r.types)
	r.types = append(r.types, t)
	return nil
}

// Get returns the media type with the given ID.
func (r *Registry) Get(id string) (MediaType, bool) {
	i, ok := r.index[id]
	if !ok {
		return MediaType{}, false
	}
	return r.types[i], true
}

// List returns all media types in registration order.
func (r *Registry) List() []MediaType {
	out := make([]MediaType, len(r.types))
	copy(out, r.types)
	return out
}
