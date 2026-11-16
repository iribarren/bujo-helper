# Feature: albums (roadmap item 12)

## Objective
Add an **Albums** media type (input `Album, Artist`) with square cover art, and make the cover sheet give square images square cells instead of shrinking them into the 2:3 column.

## Problem / Why
Album trackers are a natural extension. Album art is square, unlike books and posters.

## Licensing (read 2026-11-16)
- **iTunes Search API** (https://performance-partners.apple.com/search-api): `media=music&entity=album`, no key, about 20 calls/min. Terms: album art may be used "only to promote store content". Personal printing does not fit. The app's Apple Books and Apple TV sources already work under these terms.
- **MusicBrainz API** (https://musicbrainz.org/doc/MusicBrainz_API): no key, at most 1 request/s, and a meaningful User-Agent with a contact is required. Core data is CC0.
- **Cover Art Archive** (https://musicbrainz.org/doc/Cover_Art_Archive/API): no key. `/release-group/{mbid}/front-1200` (also 250/500) answers 307 to archive.org, 404 when there's no front image. The images carry no licence: "use the images at your own risk".
- **User decision (2026-11-16):** CAA first, iTunes fallback. The MusicBrainz User-Agent is `cover-downloader/1.0 ( https://github.com/iribarren/bullet-journal-covers )`, the public repo URL with no personal email.

## Scope / constraints
- Media type `albums`, label "Albums", creator label "Artist", date label "Listened on". Files are named `Album - Artist.jpg`.
- Sources, in order:
  1. **MusicBrainz + CAA** (`musicbrainz`): release-group search `https://musicbrainz.org/ws/2/release-group/?query=releasegroup:"…" AND artist:"…"&fmt=json&limit=…` (Lucene-escape the values, artist clause only if given). Rank by title and artist. The image is CAA `https://coverartarchive.org/release-group/{mbid}/front-1200`, with `front-500` and `front` as fallbacks. MusicBrainz requests use the User-Agent above. Add `musicbrainz.org` to `web.DefaultThrottle.MinInterval` at 1 s.
  2. **iTunes** (`apple`, reusing `Apple` with Media `music` and Entity `album`): it reads `collectionName` for the title and `artistName` for the artist, with artwork scaled to large as for books. Books keep `media=ebook` with no entity and stay byte-identical.
- Catalog registration covers the selector, sample CSV, placeholder example and CLI `-type albums` (usage derived from the registry).
- **Print sheet (Non-2:3 images convention, square part):**
  - An image is *square* when its aspect is within about ±10 % of 1 (constant plus tests). Other images keep today's 2:3 column cell. Landscape cells are left to items 14/15.
  - Cover sheet (`Grid.Layout`): cells flow left to right. A square cell is H × H (H = cover height), a 2:3 cell is today's `CellWidth` × H. The Gap stays between cells, and a row wraps when the next cell doesn't fit the usable width. Rows per page are unchanged.
  - Rows are centred horizontally. A row that isn't full is centred as if it were padded with cells like its last one, so the last row lines up with the rows above it. An all-2:3 sheet keeps exactly today's boxes, including the partial last row.
  - The other formats keep shrink-to-fit. Full page with square cells is a follow-up.
  - Bleed and crop marks work per cell, and the gap still absorbs the bleed.
- **UI:** the toolbar page count is exact for the selected images. A new binding counts the pages from the image headers; it may be async. The size labels' "N per page" shows the square capacity when every selected image is square, and the 2:3 capacity otherwise.
- **Defaults keep today's output:** every existing format, for 2:3 images, is byte-identical.
- README: Sources (albums, the licensing notes and the User-Agent) and the Print sheet (square cells).
- Delivery: branch `feat/albums` from `b8f0b85`. The forecast is about 900 authored lines (about 55 % tests), so the strategy is `ask-on-risk`, applied at the slice boundary. The user GUI-checks, then a local `--no-ff` merge. Push and merge are the user's decision.

## Acceptance criteria
- Provider tests with recorded fixtures (MusicBrainz search, the CAA redirect or 404, iTunes album).
- A sheet of only square covers fills the page with square cells. The 2:3 output doesn't change. Mixed folders lay out with no overlap and no overflow.
- The live CLI downloads a well-known album (for example `Abbey Road, The Beatles`).
- README Sources and Print sheet sections are updated.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`. If the bindings change, run `wails build -tags webkit2_41` and revert the 0755 modes it sets on `frontend/wailsjs/go/main/*` and `models.ts`. Rebuild `build/bin/cover-downloader` before the GUI check.

## Tasks
- [ ] T1 Providers: MusicBrainz + CAA provider, Apple album mode, throttle entry, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [ ] T2 Catalog `albums`, CLI, frontend example and README Sources. Route: same delegated writer, separate commit.
- [ ] T3 Sheet: square cells in the cover-sheet flow layout, page counting, tests and README Print sheet. Route: delegated writer.
- [ ] T4 UI: exact page count binding, size labels, Wails bindings and a build. Route: delegated writer.
- [ ] T5 Live CLI smoke test and build. Route: inline.

## Progress

## Next step
T1.
