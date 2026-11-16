# Feature: bujo-helper-release (pre-release cleanup)

## Objective
Prepare a release with what already works: drop roadmap item 7, rename the app to **BuJo Helper**, and add i18n with Spanish. The default language is the OS language, and it can be changed in Settings.

## Problem / Why
- Part 1 is done. Grayscale (item 7) isn't needed, because the printer's settings already do it.
- The app will offer printables beyond covers and media, so "Cover Downloader" no longer fits. The name may change again later.
- Spanish-speaking users need a translated UI.

## Decisions (user, 2026-11-16)
- **Name:** "BuJo Helper", slug `bujo-helper`. The rename covers everything, including the Go module. The user will rename the GitHub repo to `iribarren/bujo-helper`.
- **Kept:**
  - the `cmd/covers` CLI name
  - the `~/Pictures/Covers` default folder
  - the `odd/tasks` history and `legacy/`
- **Settings folder:** moves to `bujo-helper`. On first load, a one-time copy brings over the old `cover-downloader/settings.json`.
- **i18n split:**
  - The frontend translates every UI string by ID: media types, formats, sizes, papers and cut modes.
  - Go translates only native dialogs, top-level binding errors, the overwrite prompt, per-item status codes and PDF stub labels.
  - Provider and HTTP diagnostics stay in English. The CLI stays in English.
- **Language:** `settings.language` is `""` (System), `"en"` or `"es"`. The frontend resolves it with `navigator.language` when it's System, then tells Go through `SetLanguage`.
- **Translation code:** hand-rolled `t()`/`tn()`, with no new runtime dependency.

## Delivery strategy
- `ask-on-risk`. Forecast: well over 400 authored lines, most of it in T4 (translation dictionaries plus rewiring UI strings).
- We'll ask for the chain strategy only if a PR is wanted. The user usually merges locally.

## Tasks
- [x] T1 Roadmap item 7 marked WONT DO. Route: inline (one docs file).
- [x] T2 Rename to bujo-helper: module and imports, wails.json with `info`, title and h1, settings dir with migration and tests, User-Agent, release.yml, README. Route: delegated writer (many files).
- [ ] T3 Go i18n: the `internal/i18n` package with a parity test, `Settings.Language`, `SetLanguage`, translated dialogs and errors, the `isYes` fix, `ItemEvent.Code`, sheet stub/card labels with a fit test, regenerated bindings. Route: delegated writer (2+ non-trivial files).
- [ ] T4 Frontend i18n: `src/i18n` (en, es, index), `data-i18n` in index.html, main/print/settings using `t()`, the Language select, live switching, vitest tests, a CI test step. Route: delegated writer.
- [ ] T5 README Language section, final checks, release prep (v0.2.0 tag is the user's call). Route: inline.

## Acceptance
- `go test ./...` and `go vet` are clean. `npm run build` and `npm test` pass. `wails build` produces `build/bin/bujo-helper`.
- A Spanish OS gives a Spanish UI. Settings can override the language, and the change applies live and persists.
- Old settings migrate.
- Spanish PDFs show translated card and stub labels.

## Progress / evidence
- T1: ROADMAP.md item 7 is `[-] WONT DO`, and the legend has been updated.
- T2: module `bujo-helper`, imports, wails.json (`name`, `outputfilename`, `info`), window title and h1, User-Agent strings, release.yml asset names and README are renamed. Settings move to `<config>/bujo-helper/settings.json`. `settings.Migrate(legacy, path)` copies `cover-downloader/settings.json` once when the new file is missing; it never overwrites, keeps the legacy file and skips a missing, unreadable or invalid legacy file. The GUI (`NewApp`) and the CLI (`loadUserSettings`) run it. RED: `TestMigrate/copies_the_legacy_settings` (new file missing) and `TestPathIsUnderTheUserConfigDir` (old dir) failed against stubs; then GREEN. Checks: gofmt clean, `go vet` clean, `go test ./...` passes in all 14 packages, and `wails build -tags webkit2_41` builds `build/bin/bujo-helper` (wailsjs mode-only changes reverted).

- T1 review assessment: passive (docs only, `non_executable_only`), so no review was needed.
- T2 commit `9f6657d`. Review assessment: high risk (shell in release.yml), 39 files, 394 lines. The user granted consent and we ran a 4-lens review (lineage `review-baebdc535d24a222`). Result: **approved and acknowledged**. Advisory findings, none blocking:
  - R4 WARNING and R3 SUGGESTION: a legacy settings file that is unreadable or invalid is skipped silently.
  - R3 WARNING: the GUI migration wiring (`NewApp`) has no test. R3 SUGGESTION: the CLI wiring has no test either.
  - R2 SUGGESTIONS: the migration wiring is duplicated in the CLI, `productVersion` is hardcoded in wails.json, and a test uses a sentinel directory.
  - Follow-up T2b handles the two warnings.
- [ ] T2b (follow-up from review): report a skipped legacy file instead of skipping it silently (`Migrate` returns a non-fatal error that callers log or warn about), and test the GUI and CLI migration wiring.

## Next step
T3 (with T2b in a separate commit).
