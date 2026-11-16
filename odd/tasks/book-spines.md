# Feature: book-spines (roadmap item 4)

## Objective
Add a **Spines** print format for bookshelf trackers: per selected image, a narrow vertical strip filled with the cover's dominant colour, with the title (and the creator, smaller) rotated 90° in black or white, whichever contrasts better.

## Problem / Why
Bookshelf trackers (a drawn shelf filled with spines) are a very popular spread, and drawing the spines by hand is tedious.

## Scope / constraints
- Extend item 3's **Format** control: add `{"spines", "Spines", "spines", UsesSize: true}` to `sheet.Formats`. Reuse `ParseName`, `fitLines`, the Go fonts and the cut-border style.
- Spine height = the cover size preset (Mini 25, Standard 50, Large 80 mm), so the Cover size select applies.
- Width varies a little for a natural shelf look, deterministically by column position (a fixed pattern of 8–12 mm widths), so every full row holds the same number of spines and the per-page count stays constant for the UI.
- Spines in a row are separated by a small gap (~2 mm), rows by `Gap`; rows centred on the page. Grey cut border around each spine.
- Text reads top-to-bottom (English spine convention). Title bold, creator smaller; both fitted along the spine length: shrink within limits, then truncate with "…". Never overflow the strip.
- Dominant colour: computed from the image with the standard library only (downscale, quantize into buckets, take the most populous bucket's mean). Text colour: black or white by WCAG contrast ratio.
- Per-page counts: the UI needs counts for a size-based format other than the cover sheet; unify the per-page data the `PrintPapers` binding sends (format × size) instead of adding a third ad-hoc map.
- Defaults keep today's output (cover sheet unchanged). Default PDF name `spines_<paper>_<size>.pdf`.
- Delivery: branch `feat/book-spines`, forecast ~700 authored lines (≈60 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- Text readable on light and dark colours; long titles shrink or truncate to fit.
- Tests for colour extraction (solid-colour fixtures) and the contrast choice; layout tests for spine rows per paper × size.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41`.

## Tasks
- [ ] T1 sheet: spines format, dominant colour, contrast choice, spine layout, rotated-text PDF rendering; tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [ ] T2 app + UI + docs: unified per-page counts (format × size), page count for spines, README Print sheet section. Route: same delegated writer, separate commit (writer trigger: app.go, print.ts, README).

## Progress
- Feature doc created; branch `feat/book-spines`.

## Next step
T1 and T2 (one writer, two commits).
