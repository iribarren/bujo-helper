# Feature: plant-care-cards (roadmap item 33)

## Objective
A **Plant care cards** Make pages kind: a card per plant with the plant name, its plate image (a nature illustration from item 17), and labelled ruled fields for **Light**, **Water**, **Soil** and **Notes**, following the Lettering and Colour-in conventions.

## Problem / Why
Optional extension of item 17 (Vintage nature illustrations, `odd/tasks/nature.md`): a care card to keep next to a plant plate.

## Scope / constraints
- Make pages kind (not a Print covers format: formats cannot be limited to a media type, and recipe cards already give the image + card-slot model).
- Template: recipe cards (`internal/sheet/recipepage.go`): card sizes (A6, 4×6, full page), slots per paper, `printCroppedImage` for the image, `blankCard`-style label + rule.
- Options: a list of plants, each `{Name, Image, Light, Water, Soil, Notes}` plus the card `Size`. Image: absolute JPEG/PNG path picked with a file dialog binding. Name empty + image set → name from the file name via `ParseName` (nature downloads are `Common name - Scientific name.jpg`; scientific name printed small under the name when present). Field text optional: typed text prints on the rules, empty fields stay blank ruled lines to write by hand. An empty list prints blank cards (like recipes).
- Lettering: plant name drawn with the lettering font, hollow when Colour-in is on and size ≥ 14 pt; small labels/body stay in the Go font. The image itself is never altered by Colour-in.
- PDF words (`Light`, `Water`, `Soil`, `Notes`) are `PageLabels` with EN/ES in `internal/i18n/catalog.go`; UI words in `frontend/src/i18n/en.ts` / `es.ts`.
- Draw code uses only `pdf.Text`/`GetStringWidth` (textguard test). Deterministic output.
- Out of scope: Print covers output (unchanged), new paper sizes, downloading images from the card.

## Acceptance criteria
- Go: registry entry, labels, card slots per paper × size, text fits per font × paper × size, blank cards, bad image rejected, hollow title with Colour-in (`1 Tr`), name from file name.
- Frontend: plant form → options JSON, image picker wiring, EN/ES labels.
- README `### Plant care cards` under `## Generated pages` + Lettering "What uses the font" table; ROADMAP item 33 ticked.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- Bindings change: `wails build -tags webkit2_41`, then chmod 0644 `frontend/wailsjs/go/main/*` and `frontend/wailsjs/go/models.ts`; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/plant-care-cards` from `812b5b1`. Forecast ~900 authored lines (≈ 45 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `812b5b1`.
- Test-first: Go layout/fit tests and vitest form tests are runnable and deterministic → RED before GREEN.

## Tasks
- [x] T1 Go backend: `plants` kind (options, decode, slots, draw), `ChoosePlantImage` binding, `PageLabels` + i18n catalog, wailsjs regenerated, Go tests. Route: delegated writer (writer trigger: several non-trivial Go files). Commit `e5382d5`.
- [ ] T2 Frontend + docs: plant form (list editor, image picker, size), registration in `pageForms`, EN/ES UI strings, vitest tests, README, ROADMAP tick. Route: delegated writer (writer trigger: several TS files + docs).

## Progress
- Exploration done (explorer: registry `internal/sheet/pages.go:139-248`, recipes template, i18n, form patterns).
- 2026-11-24 T1 (delegated writer): `plants` kind last in `PageKinds` (`internal/sheet/plantpage.go`): `PlantsOptions{Plants, Size}` over the recipe card sizes and `recipeSlots`, plants normalised, unknown fields rejected, image absolute `.jpg`/`.jpeg`/`.png`; one card a plant, blank cards fill the last page. Layout: plate at the top (at most 40 % of the card), **contained, not cropped** (whole plate, aspect kept, centred: a cropped botanical plate loses the plant); name in the lettering via `fitRecipeTitle` (hollow with Colour-in ≥ 14 pt; a writing line when empty), from the file name via `ParseName` when not typed, the scientific name below in Go italic (`goitalic`, embedded only when one prints); label column + rules, Light/Water/Soil 2 rules each with room for 8 lines (else 1), Notes the rest; typed text on the rules via `fitLines` (cut with "…"). Shared `drawCutLines` (recipes use it too). `PageLabels.PlantLight/Water/Soil/Notes` + `pdf.plant*` (EN/ES), `dialog.plantImage`; `App.ChoosePlantImage` sharing `chooseImage` with `ChooseRecipePhoto`; wailsjs regenerated. 2 approved test-order edits outside the original surface: `calendarpage_test.go` (full kind order + `plants`) and `conversionspage_test.go` (conversions between recipes and plants). Kind name localisation (`pageKind.plants`) is frontend → T2. RED: compile failures (`undefined: Plant`, `a.ChoosePlantImage undefined`). GREEN: `go test` 3706 passed; `go vet` clean; `gofmt -l .` empty; tsc ok, vitest 553/553 (19 files); `wails build` ok. Commit `e5382d5`.

## Next step
T2.
