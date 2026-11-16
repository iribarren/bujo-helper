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
- [x] T3 Go i18n: the `internal/i18n` package with a parity test, `Settings.Language`, `SetLanguage`, translated dialogs and errors, the `isYes` fix, `ItemEvent.Code`, sheet stub/card labels with a fit test, regenerated bindings. Route: delegated writer (2+ non-trivial files).
- [x] T4 Frontend i18n: `src/i18n` (en, es, index), `data-i18n` in index.html, main/print/settings using `t()`, the Language select, live switching, vitest tests, a CI test step. Route: delegated writer.
- [x] T5 README Language section, final checks, release prep (v0.2.0 tag is the user's call). Route: inline.

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
- [x] T2b (follow-up from review): report a skipped legacy file instead of skipping it silently (`Migrate` returns a non-fatal error that callers log or warn about), and test the GUI and CLI migration wiring.
  - Route: delegated writer. `Migrate` returns an error wrapping `settings.ErrLegacySkipped` and the cause for an unreadable or invalid legacy file; a missing one is still `nil`, the new file is never overwritten and the legacy file stays. `migrateSettings` logs it; the CLI's `loadUserSettingsWarning(io.Writer)` seam warns on stderr. Tests: `TestMigrate` (skipped and reported), `TestNewAppMigratesTheLegacySettings`, `TestNewAppLogsASkippedLegacyFile`, `TestLoadUserSettingsMigratesTheLegacySettings`, `TestLoadUserSettingsWarnsAboutASkippedLegacyFile` (temp `XDG_CONFIG_HOME`/`HOME`/`AppData`).
  - RED: `TestMigrate/*_skipped_and_reported`: `Migrate = <nil>, want ErrLegacySkipped`; against the old `Migrate`, the GUI and CLI warning tests failed (`log = ""`, `warnings = ""`). The migration wiring tests are characterization tests and passed on the old code. GREEN after the change. Checks: gofmt, `go vet ./...`, `go test ./...`.
- T2b commit `9d765e8`.
- T3: route: delegated writer (2+ non-trivial files). New `internal/i18n` (`Lang`, `EN`/`ES`, `Parse`, `T` with EN → key fallback, `All`), catalogs of 26 keys (dialogs, filters, overwrite prompt, binding errors, PDF labels). `settings.Settings.Language` (`Trimmed` lower-cases it and turns anything but `en`/`es` into `""`). `App.SetLanguage` (mutex-guarded, default EN) drives every native dialog title and filter, binding error (`create output folder` still wraps its cause), the overwrite prompt, and `isYes`, which now accepts every language's Replace label. `downloader` adds the `ErrCancelled`, `ErrEmptyTitle` and `ErrNoProviders` sentinels, `ItemResult.Err` and `Code()`. `ItemEvent.Code` and `DoneEvent.Code` carry `not_found` (only when every provider said not found), `cancelled`, `empty_title`, `no_providers` or `""`. The English `Error` text is unchanged. `sheet.Options.Labels` (`json:"-"`) holds the stub Admit, Date and Where labels and the card date fallback; zero values keep English, and `ExportPDF` fills them from the UI language. English stub and card PDFs are byte-identical to before (checked against the previous code).
  - RED: i18n, settings, sheet labels and app tests first failed to compile (undefined `EN`/`Parse`/`T`, `Settings.Language`, `Labels`, `SetLanguage`/`importCSVFile` arity). `TestItemResultCodes` failed behaviourally against a stub `Code` (`Code = "", want "not_found"`, `Err = <nil>`). The parity test was mutation-checked: a `%d` swap and a missing key both fail. All GREEN after.
  - Checks: `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` 786 passed in 15 packages. `wails build -tags webkit2_41` built `build/bin/bujo-helper` and regenerated bindings (`SetLanguage`, `settings.Settings.language`, plus an unused `sheet.Labels` class); wailsjs modes were reset to 644. `npx tsc --noEmit` is clean. `ItemEvent` is an event payload, so `code` is not in `models.ts`; T4 types it by hand.

- Parent fix: a new `testmain_test.go` makes `TestMain` point `XDG_CONFIG_HOME`, `HOME` and `AppData` at a temp dir. Many root tests call the real `NewApp()`, which ran the settings migration against the user's real config folder. That earlier run migrated the real `cover-downloader` settings correctly: the TMDB key is present in `bujo-helper/settings.json`. `go test .` passes (127 tests).

- Review of `14ed445..8b5be25` (T2b `9d765e8`, T3 `c35b688`, test isolation `8b5be25`): medium risk, 20 files, 1199 lines, `slice_budget_reached`. The user granted consent and the review-reliability lens ran (lineage `review-255286f4260373c5`). Result: **approved and acknowledged**. Advisory findings:
  - R3 WARNING "PrintPages doesn't apply labels": not applicable. `PrintPages` only counts pages and draws no labels.
  - R3 SUGGESTION: test label fit across several poster ratios, plus one long label that triggers truncation. Deferred as an optional follow-up.

- T4: route: delegated writer (2+ non-trivial files). `frontend/src/i18n`: `en.ts` (flat dotted keys, `_one`/`_other` plurals, Go presets keyed by ID), `es.ts` typed as `Messages` (a missing or extra key fails `tsc`, mutation-checked), `index.ts` (`detectLang`, `setLang`/`getLang`, `t` with `{name}` and ES → EN → key fallback, `tn` via `Intl.PluralRules` with `_other` for other categories, `onLangChange`, `applyDom`, `label`/`pluralKey` by ID with the Go label as fallback). `language.ts` `useLanguage(setting)` sets the UI language and calls `SetLanguage`. `index.html` uses `data-i18n*` on all static text. main/print/settings translate every dynamic string; media, format, size, paper and cut labels by ID; the card date label is translated before it goes to Go; item status by `code`, with the English `error` only when there is no code. Settings has a Language select (System/English/Español), saved with the other fields and applied live: `applyDom` plus per-module relabel that keeps selections, typed text, results, summary and status lines. Startup: `GetSettings` → `useLanguage` → `applyDom` → modules. vitest 5.0.5 + jsdom 29.1.1 (jsdom 30 needs Node ≥ 22.22.2; local is 22.22.1), `npm test`, and a "Test frontend" step (`npm ci` + `npm test`) in release.yml before the Wails build.
  - RED: `npm test` failed with `Failed to resolve import "./en"`; GREEN: 38 tests (detectLang table, t interpolation/fallbacks, tn en/es 0/1/2, label, onLangChange, applyDom, key and placeholder parity). UI wiring has no DOM harness; it is covered by `tsc` and a manual GUI check.
  - Checks: `npm run build` clean, `npm test` 38 passed, `npm ci` + `npm test` with npm 10.9.4 pass from the lockfile, `go test ./...` 786 passed, `wails build -tags webkit2_41` built `build/bin/bujo-helper` (wailsjs modes reset to 644). The Debian npm 9.2.0 (and npm 10.9.4) `npm install` crashes on vitest's peer set (`edgesOut`), so the deps were added with `npx npm@11.6.2`.
  - Commits: `40ea155` (i18n module, tests, deps, CI), and the UI wiring commit after it.

- Review of `b999693..2f6c303` (T4 `40ea155`, `2f6c303`): high risk (shell in release.yml), 16 files, 2081 lines. The user granted consent and we ran a 4-lens review. Result: **approved and acknowledged**. Advisory findings:
  - R4 WARNING (npm 9/10 `edgesOut` crash on `npm install`): CI runs `npm ci` from the lockfile before `wails build`, and `package.json.md5` is tracked and matches, so Wails skips `npm install`. Accepted.
  - R4 SUGGESTION (a listener that throws stops the rest): fixed in `ab8d047`. RED: `keeps notifying the other listeners when one throws` failed with 'relabel failed' thrown. GREEN: 39 passed.
  - R2 (split sentence fragments, tooltip text in both the markup and the dictionary, sample titles duplicating the creator labels, the `useLanguage` name, keys built from `undefined`) and R3 (no tests for the live relabel wiring, test files not type-checked): left as follow-ups.
- T5 (inline): README has a Language section and a note on running the frontend tests. Final checks: gofmt clean, `go vet` clean, `go test ./...` 786 passed, `npm run build` clean, `npm test` 39 passed, `wails build -tags webkit2_41` builds `build/bin/bujo-helper` (wailsjs modes reset to 644).

- The user checked the GUI (2026-11-17): everything works. The GitHub repo is renamed to `bujo-helper`.
- macOS release `6499a1f`: a new `macos-latest` matrix entry builds `wails build -platform darwin/universal` and is packaged with `ditto -c -k --keepParent build/bin/bujo-helper.app` as `bujo-helper-<tag>-macos-universal.zip`. The bundle name comes from wails.json `name` (wails v2.17.1 `packager.go`: `ProjectData.Name + ".app"`). README has a macOS install section (the app is unsigned: Open Anyway or `xattr -dr com.apple.quarantine`) and the macOS settings path. The macOS job hasn't run yet; the `v0.2.0` tag run is its first test.
- Review of `2f6c303..6499a1f`: high risk (shell in release.yml). The user granted consent and we ran a 4-lens review. Result: **approved and acknowledged**. Advisory findings:
  - R3 WARNING (macOS lane not run, bundle name): the name was checked against the Wails source; the CI run will confirm it.
  - R2 WARNING (task log didn't mention macOS): fixed by this entry.
  - Suggestions left as follow-ups: R1 (pass `ref_name` through env in `run` lines; only tag pushers can trigger it), R2 (duplicated build step), R3 (move the cleanup in the new listener test into `afterEach`).

## Next step
Merge to main, push, and tag `v0.2.0`. Then check that the release workflow builds all three platforms.
