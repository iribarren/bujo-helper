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
- [x] T1 Providers: MusicBrainz + CAA provider, Apple album mode, throttle entry, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Catalog `albums`, CLI, frontend example and README Sources. Route: same delegated writer, separate commit.
- [x] T3 Sheet: square cells in the cover-sheet flow layout, page counting, tests and README Print sheet. Route: delegated writer.
- [x] T4 UI: exact page count binding, size labels, Wails bindings and a build. Route: delegated writer.
- [x] T5 Live CLI smoke test and build. Route: inline.

## Progress
- Plan commit `42d2c12`. RDD on (global). `.codegraph/` (local CodeGraph index) is excluded in `.git/info/exclude`, not committed.
- T1 `8d47b93` feat(providers): MusicBrainz cover art and iTunes album search. Route: delegated writer (2+ non-trivial files). 8 files, +696 / −12 (about 243 lines are fixture JSON).
  - `NewMusicBrainz` searches release groups. Lucene escapes the full reserved set (checked live). Album types are ordered first, then `rankCandidates` (a stable sort). The artist is artist-credit names plus join phrases. Images: CAA `front-1200` → `front-500` → `front`.
  - `web.SetMusicBrainzHeaders` sends the repo-URL User-Agent. `musicbrainz.org` is throttled to 1 s.
  - `NewAppleAlbums` uses `media=music&entity=album` and the `collectionName` title. A test checks the books URL byte for byte.
  - RED: build failed (`undefined: NewMusicBrainz`, `NewAppleAlbums`, `web.MusicBrainzUserAgent`), and `MinInterval["musicbrainz.org"] = 0s`. GREEN: providers and web, 107 passed.
- T2 `717b716` feat(catalog): albums media type. 7 files, +72 / −17. Covers the registry (Artist, Listened on, 5 samples), CLI usage and help, the frontend example and README Sources.
  - RED: `TestDefaultAppMoviesUseTheSavedTMDBKey` and `TestUsageListsMediaTypes` failed (no albums). GREEN: 120 passed.
  - The downloader moves on to the next URL, then the next candidate, on a 404. The CAA 307 redirect is followed, and the extension comes from Content-Type. No downloader change.
- T3 `ead26ae` feat(sheet): square cells for square covers. Route: delegated writer. 7 files, +689 / −37.
  - `IsSquare`: 0.9 ≤ r ≤ 1/0.9. New `Grid.Flow`, used only by the cover sheet. `Grid.Layout` is kept because Full page uses it. `Format.SquareCells` is set only on covers.
  - Every row is centred as if padded with copies of its last cell. Rows with no squares reuse `place`, so their boxes are identical to today's.
  - New `Options.Pages(ratios)`, `Options.SquarePerPage()` and `AspectRatios(paths)`.
  - Square capacity (Mini / Standard / Large): A4 54 / 15 / 6, Letter 63 / 12 / 6, A5 24 / 6 / 2.
  - RED: 8 new tests failed against stubs. GREEN: the sheet package passes. Two cover tests and the bleed tests were updated, because a square fixture now gets a square cell.
  - Byte identity: 376 renders. All 188 with no square image are identical to the base. With square images, the 107 non-cover renders are identical and the 81 cover-sheet renders differ, as expected.
- T4 `2b92cf1` feat(ui): exact page count for square covers. 7 files, +253 / −23.
  - Binding `PrintPages(paths, opts)` returns `{pages, allSquare, exact}`. An unreadable image falls back to the `ceil` estimate.
  - `PrintPaperInfo.SquarePerPage` lists the square capacity. `print.ts` shows the estimate at once, then the exact count, and the latest request wins. The size labels show the square capacity when every selected image is square.
  - Wails bindings were regenerated with modes restored to 0644, and `build/bin/cover-downloader` was rebuilt.
  - RED: `undefined: PrintPagesInfo`. GREEN: `TestPrintPages` and others.
- Writer verification: `go test ./...` 666 passed, `go vet` clean, `gofmt` empty, `tsc` clean, `wails build` ok. Parent spot check: `go test ./internal/sheet/ .` 462 passed. A4 Standard renders were reviewed: squares in a 3-wide grid with the last one aligned, mixed rows centred.
- Known open points for the GUI check:
  - A row that wraps before a square, such as album, album, book, sits about 20 mm left of centre. This follows the padding rule.
  - The size labels update after the async answer.
  - With no CAA front image, a candidate costs 3 failed image requests.
- Review of T1–T2: consent granted, but START was refused with `stale_target_identity`, because the T3–T4 commits landed in between. The review is re-derived over the whole slice `42d2c12..HEAD`.
- Review `42d2c12..30ad7ed` (27 files, 1834 lines, medium): consent granted. The review-reliability lens approved it, and it was acknowledged (authority burned). Three advisory findings:
  - R3 WARNING: the `PrintPages` fallback under-counted square covers. Fixed.
  - R3 SUGGESTION: no retry after a failed count. Fixed.
  - R3 SUGGESTION: no frontend test for the request sequencing. There is no frontend test runner, so the GUI check covers it.
- Fix `39433c7` fix(ui): an unreadable image now counts as a 2:3 cover, so readable squares keep square cells (`Exact: false`). A failed request clears `pagesKey`, so it can be retried.
  - RED: `TestPrintPages/unreadable_image` gave 2 pages, want 3. GREEN: `go test ./...` 678 passed. `go vet` clean, `gofmt` empty, `tsc` clean. `wails build` was rerun and the modes restored.
  - Assess `30ad7ed..39433c7`: medium, 43 lines, `under_budget`, so no review is due.
- T5 live CLI (keyless, `-type albums`): 6 of 6 OK.
  - 5 came from MusicBrainz + CAA at 1200 × 1200 (Kind of Blue 1200 × 1192): Rumours, Kind of Blue, Random Access Memories, El mal querer, Nevermind.
  - Abbey Road came from iTunes at 1500 × 1500. MusicBrainz answered `503 "server is currently busy"`; the throttle retried 3 times, then the item fell back to iTunes, as designed. CAA does have the Abbey Road front (checked by hand).

## Next step
User GUI check (`build/bin/cover-downloader` rebuilt at `39433c7`). Then the local `--no-ff` merge and push, if the user asks.
