# Feature: release-build-tooling (roadmap item 40)

## Objective
Close the release and build tooling leftovers: harden the release workflow, stop the `vite build` chunk warning, type-check the frontend tests, fix the i18n leftovers, and add the missing label fit test.

## Problem / Why
Leftovers from the release work (ROADMAP item 40; sources `odd/tasks/bujo-helper-release.md`, `boardgames.md`, `live-preview.md`).

## Scope / constraints
Mapped by a read-only explorer.

- Workflow (`.github/workflows/release.yml`): no warning when `BGG_TOKEN` is empty (`boardgames.md:54`); no post-build check that the `-X` symbol was linked (Wails strips symbols with `-w -s`, so grep the binary for the token bytes, as checked by hand in `boardgames.md:55`, without printing it); `matrix.asset` (built from `ref_name`) is inlined in `run` lines `:83,88,93` → pass through env; Linux and macOS build steps are the same bash command (`bujo-helper-release.md:82`) → one bash build step for all three OSes; `productVersion` hard-coded `0.2.0` in `wails.json` → stamp it from the tag in the workflow.
- Frontend chunk: no `vite.config.*`; pdf.js is imported statically by `frontend/src/preview.ts:14-15`, so it lands in the 570 kB main chunk (`live-preview.md:52`) → `manualChunks`.
- Test files not type-checked (`tsconfig.json` excludes `src/**/*.test.ts`): 42 errors in 15 files (no `@types/node`, `__dirname`, vitest 5 `vi.fn()` typing, `NodeListOf` iteration without `DOM.Iterable`, one unused import).
- i18n (`bujo-helper-release.md:74`): key tip split into fragments (`index.html:17-21`, `i18n/en.ts:21-30`, `main.ts:84-110`); English `title` in markup next to `data-i18n-title` (`index.html:71,122,127,132,137,153,156,196,199`); keys built from `undefined` (`main.ts:284-285`, `:391`).
- Listener test cleanup inline (`frontend/src/i18n/i18n.test.ts:99-127`) → `afterEach`.
- Label fit (`bujo-helper-release.md:64`): `TestLabelsFitTheirBoxes` (`internal/sheet/labels_test.go:45`) uses one ratio `0.656` and no truncating label → several poster ratios plus one long label.

Out of scope: building the CLI in the release, a dynamic pdf.js import, other i18n items from the same note (sample titles, `useLanguage` name).

## Acceptance criteria
- `vite build` has no chunk warning.
- The workflow passes a YAML parse check (`actionlint` is not installed).
- Tests for the i18n fixes; frontend tests are type-checked in `npm test`.
- ROADMAP item 40 ticked.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- `cd frontend && npx tsc --noEmit && npm test && npx vite build`; delete `frontend/.vitest/` if created.
- Workflow: `python3 -c 'import yaml,sys; yaml.safe_load(open(sys.argv[1]))' .github/workflows/release.yml`

## Delivery
- Branch `feat/release-build-tooling` from `30acc79`. Forecast ~400 authored lines (excluding `package-lock.json`). Strategy `single-pr` (cached project choice: user checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `30acc79`.
- Test-first: vitest and Go tests are runnable → RED before GREEN for the i18n fixes; the type-check errors are the RED for T3; workflow and Vite config have no runnable test (YAML parse and `vite build` output instead).

## Tasks
- [x] T1 Release workflow: empty-secret warning, post-build token check, `ref_name`/asset through env, one build step, `productVersion` from the tag. Route: inline (one mechanical file). Commits `6ea15f3`, `8fe290c`.
- [x] T2 Vite config: `manualChunks` for pdf.js, no chunk warning. Route: inline (one new config file). Commit `3ec2283`.
- [x] T3 Type-check test files: `@types/node`, `tsconfig.test.json`, fix the 42 errors, run it in `npm test`. Route: delegated writer (writer trigger: many test files). Commit `26e0a50`.
- [x] T4 i18n fixes + listener `afterEach`, with tests. Route: delegated writer (several non-trivial files). Commit `c9a536f`.
- [x] T5 Label fit test across poster ratios + long label; ROADMAP tick. Route: delegated writer (needs reading sheet layout code). Commit `712384d`.

## Progress
- 2026-11-25 Exploration done (explorer, path:line evidence above).
- 2026-11-25 T1 done (route: inline). One bash `Build` step for all three OSes (`-tags` from the matrix), with a `::warning` when `BGG_TOKEN` is empty; `-trimpath` added, because without it Go records `-ldflags` (and so the token) in the build settings and the token bytes are in the binary even when the `-X` symbol name is wrong (local probe with `cmd/covers`: mislinked 2 hits, linked 3; with `-trimpath` 0 and 1). `Check the token was linked` greps the binary (`grep -qaF`, prints nothing; path from `matrix.binary`). Asset names and the binary path go through env. `Set the version from the tag` runs first: `v1.2.3` and `v1.2.3-rc1` → `1.2.3`, anything else fails (local run: `v`, `v1.2`, `vfoo` exit 1); `wails.json` keeps `0.0.0`. README releasing text updated. Checks: YAML parse ok (`actionlint` not installed); version script run on 5 tags.
- 2026-11-25 T1 review (`30acc79..6ea15f3`): high, consent granted, 4 lenses approved, acknowledged, authority burned (lineage review-148da1ad11abcf6e). Advisories: tag version unvalidated (R3/R4 warnings) and duplicated binary path (R2) → fixed in `8fe290c`; stale task progress (R2) → this update; workflow behaviour unproved outside a real tag push (R3) → follow-up.
- 2026-11-25 T2 done (route: inline). `frontend/vite.config.ts`: pdf.js in its own `pdfjs` chunk; the legacy build alone is 507 kB, so `chunkSizeWarningLimit: 600` with the reason in a comment (the app loads from disk). `vite build`: before `index` 634 kB + warning; after `index` 127 kB, `pdfjs` 507 kB, no warning. `vitest run`: 575 passed.
- 2026-11-25 Review `6ea15f3..3ec2283` (T1 fix + T2): high, consent granted, 4 lenses approved, acknowledged, authority burned (lineage review-1c6141a3f5666f4f). Advisories: `matrix.binary` and the tag-only trigger are outside the diff (both defined in the file; no change). Reviewed boundary → `3ec2283`.
- 2026-11-25 T3 done (route: delegated writer). `@types/node` 22.20.5 (exact); `frontend/tsconfig.test.json` extends `tsconfig.json`, includes the tests, adds `DOM.Iterable` and `node` types there only; mocks typed with `vi.fn<Sig>()`; `npm test` = `tsc -p tsconfig.test.json && vitest run`. RED: 42 errors (14 TS2304, 14 TS2307, 10 TS2345, 3 TS2488, 1 TS6133); GREEN: 0.
- 2026-11-25 T4 done (route: delegated writer). One whole sentence per provider (`tip.tmdb/igdb/bhl/bgg`, `{settings}` placeholder) in en and es, rendered as text nodes around the existing Settings button (no innerHTML); 10 fragment keys removed. 9 duplicated English `title` attributes removed; `applyDom` sets them at startup (tested). `label`/`pluralKey` return the fallback for an undefined ID; `updateSampleButton` and `failureText` guard before building keys. Listener tests clean up in `afterEach`. RED: 10 failing tests (sentences, markup titles, `label`/`pluralKey` fallback); the two `main.ts` key sites had no runtime RED (the old guards already stopped the lookup) and their tests are regression guards. GREEN: `tsc` (app and tests) clean, 20 files / 586 tests passed, `vite build` no warning. Parent spot check: `npm test` 586 passed.
- 2026-11-25 Review `3ec2283..c9a536f` (T3+T4): medium, `slice_budget_reached`, consent granted, reliability lens approved, no advisories, acknowledged, authority burned (lineage review-2f457b5c6eb50292). Reviewed boundary → `c9a536f`.
- 2026-11-25 Stop hook candidate (uncommitted feature doc): low, auto-approved, acknowledged (lineage review-c3f28276f24db3dc).
- 2026-11-25 T5 done (route: delegated writer). Test-only, green first run (pins behaviour). `internal/sheet/labels_test.go`: `TestLabelsFitTheirBoxes` over six poster ratios (16:9, square, 0.675, 0.656, 1:2, 3:10) × en/es; new `TestStubDrawsLabelsThatFit` draws a real stub (uncompressed PDF) and reads back labels and rules: en/es print whole, a long Date label is cut to a prefix ending in `…`, fits `Date.W - labelRuleGap - minDateRule`, and the rules keep at least `minDateRule`. Mutation check: without the width reservation or without truncation, the 6 long-label subtests fail ("2.318… mm left for the rules, want at least 10"). Note: label boxes come from a fixed 2:3 cover area, so they do not depend on the ratio today. GREEN: `go test -tags webkit2_41 ./...` ok, `go vet` clean, `gofmt -l .` empty; parent spot check `go test ./internal/sheet -run Label` 48 passed. Assess `--base-ref c9a536f --committed-only`: medium, `under_budget` (pending in slice).
- 2026-11-25 ROADMAP item 40 ticked with a "Done in branch" block; the merge hash is added at merge.
- 2026-11-25 Whole-branch review (`30acc79..aa94fa8`): high, consent **declined** by the user for this candidate (T1–T4 reviewed individually; T5 is test-only). This record stays uncommitted until the merge.

## Follow-ups
- `frontend/src/print.ts:91` builds `print.headerExample.${formatId}` before a guard (looked up as `print.headerExample.` when the ID is empty; harmless fallback). Guard it like `label()`.
- Pager buttons carry both `aria-label` and `data-i18n-aria-label` (same duplication as the titles).
- Run the release workflow once before the next real release (a throwaway `v0.0.0-test` tag on a fork, or a `workflow_dispatch` dry run) to prove the bash build on Windows, the binary paths and the token check.
- 2026-11-25: user asked for the merge. Merged to main locally in `83d5f40`.

## Next step
Done. Open follow-ups: see **Follow-ups**.
