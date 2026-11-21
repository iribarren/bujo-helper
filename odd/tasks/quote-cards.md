# Feature: quote-cards (roadmap item 27)

## Objective
A **Quote cards** page kind in Make pages. The user searches an author or personality, the app fetches that person's quotes from Wikiquote, the user ticks the quotes they want, and the app lays them out as cut-out cards in a few typographic styles, in the lettering font. The user can also type their own quotes.

## Problem / Why
Quotes and lyrics pages are common bujo collections. Typing quotes by hand is the slow part; searching a person and ticking quotes makes the page kind worth having (the roadmap's "free versions are everywhere" weakness).

## Licensing (checked 2026-11-21)
- Source: **Wikiquote** (en and es) through the MediaWiki Action API. No key. Other candidates rejected: Quotable (service unreliable), ZenQuotes and API Ninjas (author search needs a paid key), Wikidata P1683 (too sparse).
- Text licence: CC BY-SA (footer: CC BY-SA 4.0; es policy body still says 3.0). Attribution needed: the PDF prints a "Quotes from Wikiquote (CC BY-SA 4.0)" caption on each page that has a Wikiquote quote. The selection and arrangement carry Wikiquote's licence; the user picks single quotes, the app never prints a whole page's list.
- Many quotes come from copyrighted works and are on Wikiquote as fair use (short quotations). Printing a few for a personal journal stays within that; the README says so.
- API etiquette: descriptive User-Agent with contact (`web.SetWikimediaHeaders`, already used for Wikidata/Commons), serial requests, GET + JSON. Sources: en Wikiquote:Copyrights, es Wikiquote:Política de derechos de autor, Wikimedia User-Agent policy, mediawiki.org API:Etiquette.

## Decisions
- Kind id `quotes`, label "Quote cards" / "Tarjetas de citas", file tag `quotes`, `UsesTitle: true`, `UsesCut: false` (cards have their own dashed cut lines). Registered after `habits`, before `map` (canvas order: … Habits, Quotes, Map).
- Options: `quotes` ([]{`text`, `author`, `source`}; `source` is `""` (typed) or `"wikiquote"`), `style` (`classic` | `bold` | `framed`, default `classic`), `perPage` (4 | 6 | 8, default 6; two columns). At most 200 quotes; text at most 600 runes, author at most 100 runes, else an error. Control characters and space runs become one space, trimmed; a quote with empty text is dropped. No quotes → one page of empty card frames (to write by hand).
- Default title "Words to keep" / "Palabras para guardar" (`PageLabels.QuotesTitle`). Credit caption `PageLabels.QuotesCredit`, in the Go font at the page foot, only on pages with a Wikiquote quote.
- Card: quote text in the lettering font, auto-fitted (wrap, shrink from a max size down to a floor, then ellipsize); hollow with Colour-in at ≥14 pt (existing lettering rules). Author in the Go font below ("— Author"). Styles: Classic (centred, curly quotes), Bold (left-aligned, larger, author right-aligned), Framed (centred inside a thin inner frame).
- Search: new `internal/quotes` package. `Search(ctx, query, lang)`: MediaWiki full-text search on `<lang>.wikiquote.org` (namespace 0) → best title + up to 5 other matches; `action=parse` HTML of the best title → quotes = text of top-level `ul > li` (nested lists, references and edit links removed) under h2/h3 sections, skipping sections such as Disputed, Misattributed, Attributed, Quotes about…, See also, External links, References (and the Spanish equivalents). 10–400 runes, deduplicated, at most 60. Language follows the UI (en or es).
- Binding `SearchQuotes(query string) (quotes.Result, error)` on `App`; result `{person, url, matches[], quotes[{text, author, source}]}`. Picking another match calls it again with that title.
- Form (canvas Make pages + user request): "Find quotes by" input + Search button; resolved person with "Other matches" buttons; results as a checkbox list (all · none, count); "Your own quotes: Quote, Author" textarea (split on the last comma, as the canvas); Style segmented; Cards per page segmented. Sends ticked results followed by typed quotes.

## Scope / constraints
- Extend, don't replace: entry in `PageKinds`, form in `pageForms.ts`; `PageKinds`/`PagePages`/`ExportPage`/`PreviewPage` unchanged. One new binding (`SearchQuotes`).
- Dot grid, moon, calendar, habits, map and Print covers output unchanged.
- EN + ES strings for every label.
- Out of scope: CSV/Markdown import of quotes, other quote sources, lyrics, saving search results between sessions.

## Acceptance criteria
- Provider tests with recorded Wikiquote fixtures (en and es): a well-known person returns quotes; excluded sections are skipped; an unknown person gives a clear "no quotes found" error.
- Text auto-fits each card without overflow on every paper, style and per-page value (table-driven layout tests); pagination; credit caption only with Wikiquote quotes.
- Frontend tests: search, picking, typed quotes, options sent.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `wails build -tags webkit2_41`; reset 0644 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/quote-cards` from `777a3b8`. Forecast ~2000 authored lines (≈45 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `777a3b8`.

## Tasks
- [x] T1 Wikiquote search: `internal/quotes` (client, HTML parsing, section filter, cleaning, limits) with recorded fixtures; `SearchQuotes` binding in `app.go` with a test; regenerated bindings. Route: delegated writer (writer trigger: 3+ non-trivial files).
- [ ] T2 Page kind `quotes`: `internal/sheet/quotespage.go`, registry entry, `QuotesTitle`/`QuotesCredit` labels wired from i18n; tests. Route: delegated writer.
- [ ] T3 Make pages form: search, results checklist, own quotes, style, cards per page; i18n EN/ES; vitest. Route: delegated writer.
- [ ] T4 Docs + verify: README Quote cards section with the licence findings, ROADMAP item 27 notes, full checks, `wails build`. Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/quote-cards` from `777a3b8`.
- T1 done (delegated writer). Commits: `8c95a68` feat(quotes): Wikiquote search; `68fe75a` feat(app): SearchQuotes binding. ~1050 authored lines (≈56 % tests; fixtures and generated bindings excluded): over the 400 heuristic because the parser, client and their table tests form one unit.
  - RED: `go test -tags webkit2_41 ./internal/quotes/` → build failed (undefined: maxQuotes, excludedSection, …); `go test -tags webkit2_41 . -run SearchQuotes` → build failed (a.quotes, a.SearchQuotes undefined).
  - GREEN: both pass; `go test -tags webkit2_41 ./...` ok, `go vet` clean, `npx tsc --noEmit` clean, no mode changes since `777a3b8`.
  - Fixtures (`internal/quotes/testdata/`, recorded 2026-11-21 with the contact UA): en/es search "Einstein", en/es parse "Albert Einstein" (en trimmed to 156 KB: figures dropped, ≤4 lists/items per section, every section kept), en no-hit search.
  - Decisions: headings wrapped in frame divs (en Disputed/Misattributed sit inside `div.disputed-begin-frame`) are walked into so sections still switch. Generic headings (See also, Notes, References, Sources, Bibliography, Further reading and ES equivalents) match exactly, so "Notes from Underground" stays; Disputed/Misattributed/Attributed/Quotes about/About …/Citas sobre/Sobre …/Atribuidas/Dudosas match by prefix, so "Attributed in posthumous publications" is excluded too. Wrapping marks stripped: `" “” „“ «» ‘’`; a full stop after the closing mark (Spanish «…».) moves inside; text with several quoted parts is left alone. `<br>` becomes a space. Matches exclude both the searched best title and the resolved title. Added `ErrEmptyQuery` and i18n keys `err.noQuotes` (`{query}` replaced) and `err.quoteQuery`. `App.quotes` (`*quotes.Client`) is built from the app's HTTP client; SearchQuotes uses a 20 s timeout from `a.ctx` (Background in tests).
  - Next: T2 page kind `quotes`.
- Fixture trim (`99ff3b7`): the review tool refused T1 over its context budget, so the parse fixtures were cut to a few real items per section (en 157.8 KB → 19.7 KB, es 90.5 KB → 10.3 KB; testdata 249 KB → 31 KB), keeping the MediaWiki JSON shape, `.mw-parser-output`, TOC, every h2/h3 heading, frame-wrapped Disputed/Misattributed, excluded sections with 1–2 items, `sup.reference` and nested source lists, and every asserted quote/absent text. No test changes.
- Review `777a3b8..6701c20` (T1 + fixture trim, medium, 16 files, 1182 lines; `slice_budget_reached`): the first try failed with `lens_context_budget_exceeded` (≈250 KB of fixtures), fixed by `99ff3b7` trimming them to 31 KB. Consent granted; reliability lens approved; acknowledged (authority burned). Advisories folded into T2: R3-001 (WARNING: a best title with no quotes drops the other matches, so the user can't pick another person; try the next titles and return the matches with empty quotes instead of only an error), R3-002 (tests read request logs written by the httptest handler without the lock; race-prone). Parent spot check: `go test ./internal/quotes/` 32 passed. Reviewed boundary `6701c20`.
