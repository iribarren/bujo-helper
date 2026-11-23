# Feature: recipe-cards (roadmap item 22)

## Objective
A **Recipe cards** page kind in Make pages: the user types recipes (title, servings, time, ingredients, steps, an optional photo of their own) or imports them from a Markdown file, and the app lays each recipe out as a card at A6, 4 × 6 in or full-page size. A companion **Kitchen conversions** page kind prints a generated chart (volume, weight, oven temperatures, cups to grams).

## Problem / Why
Recipe collections are a classic bujo spread. The roadmap marks the item weak because the only image source (TheMealDB) has licensing problems; the better version downloads nothing.

## Licensing (checked 2026-11-23)
- TheMealDB (https://www.themealdb.com/api.php, /terms_of_use.php, /faq.php): the free key "1" is for "development of your app or for educational use"; "you must become a supporter if releasing publicly on an appstore". No licence is named for images ("Most of our artwork is custom and is created by our users"; a `strCreativeCommons` tag is mentioned but its licence and the rest are not). Printing images is not addressed.
- Decision: **TheMealDB is left out** (roadmap rule: only if the licensing step confirms the images may be used). No network source; photos are the user's own files.

## User decisions (2026-11-23)
- Kitchen conversions: yes, its own page kind.
- Import: Markdown only (CSV is awkward for multi-line ingredient/step lists).

## Decisions
- Kind `recipes`, label "Recipe cards" / "Fichas de recetas", file tag `recipes`, `UsesTitle: false` (each card prints its recipe's title in the lettering font; Font and Colour-in still apply), `UsesCut: false` (cards have their own dashed cut lines, like quote cards). Registered after `map`.
- Options: `recipes` ([]{`title`, `servings`, `time`, `ingredients` []string, `steps` []string, `photo` absolute path or ""}), `size` (`a6` | `4x6` | `page`, default `a6`).
- Cards are landscape like index cards: A6 148 × 105 mm, 4 × 6 in 152.4 × 101.6 mm, tiled within MinMargin margins with Gap, centred; a card that doesn't fit the paper is scaled down (aspect kept) to one per page. `page`: one recipe per page, the paper's content area, no cut line.
- Card content: title band (lettering font, fitted, hollow with Colour-in at ≥14 pt), a meta line "Serves 4 · Time 30 min" (Go font), then two columns: Ingredients (bullets, with the photo centre-cropped above it when set) and Steps (numbered). Body text in the Go font shrinks from a max size to a floor, then the rest flows onto continuation cards marked "(continued)".
- No recipes → blank recipe cards (headings and writing lines) to fill in by hand.
- Limits (else an error naming the recipe by 1-based index): 50 recipes; title 100 runes; servings and time 40; at most 60 ingredients of 200 runes and 40 steps of 600 runes; photo path 4096 bytes. Control characters and space runs become one space; blank lines are dropped; a recipe with no title, ingredients, steps or photo is dropped.
- Photo: JPEG or PNG read at draw time; a missing or unreadable file is an error that names the recipe (shown in the preview).
- Labels in the UI language through `PageLabels` (Ingredients, Steps, Serves, Time, continued, the conversion chart words).
- Markdown import: `# Title` starts a recipe; `Serves:`/`Servings:`/`Raciones:` and `Time:`/`Tiempo:` lines; `## Ingredients`/`## Ingredientes` list; `## Steps`/`## Method`/`## Pasos`/`## Preparación` list (bullets or numbers); optional `Photo:`/`Foto:` path, relative to the Markdown file. Binding `ImportRecipes()` opens a file dialog and returns the parsed recipes; `ChooseRecipePhoto()` returns an image path.
- Kind `conversions`, label "Kitchen conversions" / "Conversiones de cocina", no options, `UsesTitle: true`, default title "Kitchen conversions" / "Conversiones de cocina". Tables: volume (cup/tbsp/tsp/fl oz → ml), weight (oz/lb → g), oven (°C, °F, gas mark), cups to grams for common ingredients. Fractions written as 1/3 etc. unless the Go font has the glyph.

## Scope / constraints
- Extend, don't replace: entries in `PageKinds`, forms in `pageForms.ts`; `PageKinds`/`PagePages`/`ExportPage`/`PreviewPage` unchanged. New bindings: `ImportRecipes`, `ChooseRecipePhoto`.
- Every other page kind and Print covers output unchanged.
- EN + ES strings for every label.
- Out of scope: TheMealDB or any recipe source, CSV import, a per-photo crop offset, recipe ratings, saving recipes between sessions.

## Acceptance criteria
- Text never overflows a card: long ingredient lists shrink to the floor, then flow onto continuation cards (table-driven layout tests over sizes, papers and fonts).
- Markdown parser tests (EN/ES headings, several recipes, relative photo paths, malformed input).
- Conversion values tested against reference figures; both kinds render on every paper.
- Frontend tests: editing, adding/removing recipes, import, photo, size; options sent.
- README shows the Markdown input format.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `wails build -tags webkit2_41`; reset 0644 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/recipe-cards` from `02aa61e`. Forecast ~3000 authored lines (≈50 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge). Review slice per task.
- RDD on (global). First reviewed boundary: `02aa61e`.

## Tasks
- [x] T1 Page kind `recipes` (Go): options, normalising, limits, card sizes and tiling, title band, columns, fitting and continuation cards, photo, blank cards, labels wired from i18n; tests. Route: delegated writer (writer trigger: 3+ non-trivial files).
- [ ] T2 Markdown import + photo picker: parser with tests, `ImportRecipes` and `ChooseRecipePhoto` bindings with tests, regenerated bindings. Route: delegated writer.
- [ ] T3 Page kind `conversions` (Go): tables, labels EN/ES, layout on every paper; tests. Route: delegated writer.
- [ ] T4 Make pages forms: recipe list + editor, Import…, photo, card size; conversions form; i18n EN/ES; vitest. Route: delegated writer.
- [ ] T5 Docs + verify: README Recipe cards (Markdown format) and Kitchen conversions sections, TheMealDB finding, ROADMAP item 22 notes, full checks, `wails build`. Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/recipe-cards` from `02aa61e`.
- T1 done (delegated writer): `b41e92e` feat(sheet): recipe cards page kind; `9934ed6` feat(app): recipe card labels in the UI language. 1313 authored lines (1304 + / 9 −; ~560 tests): over the ~400 heuristic because the kind is one indivisible behaviour (options, layout, flow, photo, blank cards) with its table-driven tests.
  - RED: `go test -tags webkit2_41 ./internal/sheet/` → build failed (`undefined: Recipe`, `RecipesOptions`, `recipeCard`, `recipePart`, `MaxRecipeTitleRunes`…); `go vet .` → `en.RecipeIngredients undefined`. First GREEN run then failed only `TestMapRegistryEntry` (map no longer last) and a fixture whose time ended in a space; both fixed in the tests.
  - GREEN: `go test -tags webkit2_41 ./...` 3106 passed; `go vet -tags webkit2_41 ./...` clean; `cd frontend && npx tsc --noEmit` clean; no mode changes since `5614e96`.
  - Decisions on open details: card text sizes a6/4x6 title 16→9 pt, body 9→6 pt, meta 8 pt, headings 8.5 pt; page size title 20, body 11, meta 9.5, headings 10. 4 mm card padding; columns 40 % ingredients / 60 % steps, 4 mm apart. Title centred, fitted on one line, cut with "…" before " (continued)" at the floor. Meta line centred; no room taken when servings and time are empty; first card only. Photo fills the ingredients column width, height min(width × 3/4, 45 % of the column), centre-cropped. Body shrinks until both columns fit the first card; at 6 pt it flows line by line (an item may split across cards; numbers continue). Continuation cards: title + "(continued)", no meta, no photo; a column heading prints only on cards where that column has lines. Cards scaled down on A5 (a6 and 4x6 do not fit within the margins): layout runs in the smaller box, sizes unscaled. Leftover slots on the last page are blank cards (like quote cards' empty frames). Blank card: title writing line, "Serves ___ Time ___", both headings over writing lines at 1.6 body line heights. Photo path: only trimmed (no space collapsing), must be absolute; a photo error reads "recipe cards recipe N photo <base name>: <cause>". `pagesOn` measures only the Go font (title band height is fixed, so fonts never change the count). `RecipeContinued` label "continued" / "continuación".
- Next: T2.
