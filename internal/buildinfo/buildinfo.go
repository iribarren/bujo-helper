// Package buildinfo holds values injected into release builds.
package buildinfo

// BGGToken is the BoardGameGeek XML API application token built into the
// app. Release builds inject it at build time from the CI secret with
//
//	-ldflags "-X bujo-helper/internal/buildinfo.BGGToken=..."
//
// It is empty in source builds; the token is never committed.
var BGGToken string
