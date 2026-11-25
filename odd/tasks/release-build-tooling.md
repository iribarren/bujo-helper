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
- [ ] T1 Release workflow: empty-secret warning, post-build token check, `ref_name`/asset through env, one build step, `productVersion` from the tag. Route: inline (one mechanical file).
- [ ] T2 Vite config: `manualChunks` for pdf.js, no chunk warning. Route: inline (one new config file).
- [ ] T3 Type-check test files: `@types/node`, `tsconfig.test.json`, fix the 42 errors, run it in `npm test`. Route: delegated writer (writer trigger: many test files).
- [ ] T4 i18n fixes + listener `afterEach`, with tests. Route: delegated writer (several non-trivial files).
- [ ] T5 Label fit test across poster ratios + long label; ROADMAP tick. Route: delegated writer (needs reading sheet layout code).

## Progress
- 2026-11-25 Exploration done (explorer, path:line evidence above).

## Next step
T1.
