// Package catalog wires the built-in media types to their providers.
// Adding a media type means adding its providers and one Register call here.
package catalog

import (
	"net/http"

	"cover-downloader/internal/media"
	"cover-downloader/internal/providers"
	"cover-downloader/internal/web"
)

// Default returns the registry of built-in media types. A nil client gets
// a default one with a request timeout.
func Default(client *http.Client) *media.Registry {
	if client == nil {
		client = web.NewClient()
	}
	reg := media.NewRegistry()
	mustRegister(reg, media.MediaType{
		ID:           "books",
		Label:        "Books",
		CreatorLabel: "Author",
		Providers: []media.Provider{
			providers.NewApple(client),
			providers.NewOpenLibrary(client),
			providers.NewAmazon(client),
		},
	})
	return reg
}

func mustRegister(reg *media.Registry, t media.MediaType) {
	if err := reg.Register(t); err != nil {
		panic(err)
	}
}
