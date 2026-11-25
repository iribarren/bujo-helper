# Roadmap: bullet journal printables

New printables and content types for bullet journal users, based on research into common bujo spreads.

- **Part 1 (items 1–13):** media spreads, such as reading logs, bookshelf trackers, movie and TV trackers and sticker sheets. Done, except item 7 (won't do).
- **Part 2 (items 14–28):** other bujo collections, such as art, travel, nature, food and personal memories. Items are grouped by how well they fit the app: **Strong**, **Medium** and **Weak**.
- **Part 3 (items 29–31):** app structure and lettering. A **Make pages** section for generated printables, a live PDF preview, and lettering fonts with hollow letters for colouring in.
- **Part 4 (items 32–41):** follow-ups and possible features found during the reviews of Parts 1–3.

## How to use this roadmap

- Each item is **self-contained** and sized for **one fresh Claude session**. Start with: *"Implement roadmap item N from ROADMAP.md."*
- Within each part, items are ordered by **value first, then effort**; in Part 2 they are also grouped by fit. Any order works, with one exception: item 29 (Make pages section) must be done before the generated printables (items 16, 18, 27, 28). Item 30 (preview) and item 31 (lettering) are strongly recommended before them too.
- When an item is done, mark it `[x]` and add the merge commit hash. Items we decided not to build are marked `[-] WONT DO`, with the reason.

## Shared conventions

Any item may be the first one to touch a shared piece, so each item follows these rules:

- **Extend, don't replace.** If an earlier item already made something configurable (page size, cover size, a format selector), add to it and don't rebuild it.
- **Print format selector.** Items that add a new printable (cards, spines, stubs…) put it under a **Format** control in the Print sheet tab. If the control doesn't exist yet, create it with **Cover sheet** (today's behaviour) as the default.
- **Metadata from file names.** The print sheet only sees image files, and downloads are named `Title - Creator.jpg` (with `-2`, `-3` suffixes for duplicates). An item that needs a title or creator parses it from the file name. If a parser already exists in `internal/sheet`, reuse it; otherwise add one there, with tests.
- **Defaults keep today's output.** With default settings, the existing A4 cover sheet keeps exactly today's layout: 50 mm covers, 5 × 5 grid, grey cut border.
- **Non-2:3 images.** Artworks, photos, flags and illustrations come in any shape. Any item that adds such images makes sure the print sheet places them correctly (landscape and square cells, not only shrink-to-fit into a 2:3 column). If an earlier item already did this, reuse it.
- **Attribution.** Some sources require credit (for example Wikimedia Commons CC BY images, or the Biodiversity Heritage Library). Store the credit and licence next to each download (for example a `<file>.credit.txt` sidecar) and offer a credits page or caption in the PDF. Whichever item first needs this creates it; later items reuse it.
- **Generated printables.** Some items create a page without downloading anything (map, moon calendar, quotes, trackers). They live in the **Make pages** section (item 29), not under the Print covers **Format** control. Each one is a page kind with its own typed options form, and it reuses the shared **Page** controls (paper, cut, credits) and **Lettering** controls (item 31).
- **Colour-in mode.** Generated pages that can be coloured by hand (moon outlines, map outlines, hollow lettering) use one shared **Colour-in** toggle, which means outlines only and no fills. Don't add a separate "outline only" switch per page.
- **Lettering.** Any title or display text a page prints (headers, month names, quote text) uses the font chosen in the **Lettering** controls (item 31). Small body text, such as captions, dates and credits, stays in the default Go font for legibility.
- **UI mockups.** The UI for items 29–31, and for the forms of the generated printables, follows the design canvas at https://claude.ai/artifact/6gdFFavUM9kyA76MsCpB1R. It has four artboards: Download, Print covers, Make pages and Lettering picker. Read it with the Artifact tool's `read` action before building UI. It reuses `style.css` tokens and components; don't copy its markup. Where the canvas and this file disagree, the canvas wins for layout and this file wins for behaviour and scope. The canvas lists all six page kinds only to show the end state.
- **Licensing check first.** For any new source, the session starts by reading the source's official terms and API docs, and records the findings in the README. If the terms forbid the intended use, stop and report it to the user before writing code.
- **Sections shown.** A new tab, or a new list the user picks from (such as a new selector), joins `SECTIONS` in `frontend/src/view.ts`, so users can hide it in **Settings**; a test fails when a tab has none. Items Go adds to an existing list show up there on their own.
- **Docs and tests ship with the code.** Update the README's [Print sheet](README.md#print-sheet) section (or [Sources](README.md#sources) for media types). Add table-driven Go tests next to the layout code.

Key files: `internal/sheet/layout.go` (grid and boxes), `internal/sheet/pdf.go` (PDF via fpdf), `frontend/src/print.ts` (Print sheet tab), `app.go` (Wails bindings), `internal/media`, `internal/catalog`, `internal/providers` (media types and sources).

---

# Part 1: Media spreads

## 1. [x] Cover size presets (432db6a)

**Why:** 5 cm is too big for "year in books" grids and too small for a featured cover. Small covers are the most requested way to fit many items on one spread.

**Scope**
- Presets: **Mini** (25 mm tall), **Standard** (50 mm, default), **Large** (80 mm).
- The grid (columns, rows, column width, margins) is computed from the cover height and the page size, so pages stay full and centred.
- A size selector in the Print sheet tab. The page count in the toolbar updates live.

**Acceptance**
- Standard gives today's layout exactly.
- Mini fits clearly more covers per page; Large fits fewer. Every cover keeps its aspect ratio and the shrink-to-fit rule.
- Layout tests for each preset.

**Out of scope:** a free-form size input.

## 2. [x] Paper sizes: A4, US Letter, A5 (313c0cf)

**Why:** US users print on Letter, and many bullet journals are A5, so people want sheets that match the journal page.

**Scope**
- A paper selector in the Print sheet tab: **A4** (default), **US Letter** (215.9 × 279.4 mm), **A5** (148 × 210 mm).
- Layout constants become per-page-size inputs. Margins, gap and cover height stay the same; columns and rows are recomputed.
- The fpdf page format follows the selection.

**Acceptance**
- A4 gives today's output. Letter and A5 produce full, centred grids with no clipped covers.
- Layout tests per paper size. The README table explains how the grid is derived.

## 3. [x] Review cards (b529705)

**Why:** a cover with a rating, a date and a few notes is the most common reading-log element.

**Scope**
- New print format **Review cards**: each card has the cover on the left, then the title and creator (from the file name), an empty "Date" field, 5 empty stars and 3–4 writing lines.
- A fixed card size that fits common bullet journals (for example 90 × 55 mm, business-card-like), tiled on the page with cut borders.
- Labels work for both media types: books ("Read on") and movies ("Watched on").

**Acceptance**
- PDF export works for books and movies. Long titles wrap or are truncated with an ellipsis and never overflow the card.
- Tests for card layout and text fitting.

## 4. [x] Book spines for bookshelf trackers (430f671)

**Why:** bookshelf trackers (a drawn shelf filled with spines) are a very popular spread, and drawing the spines by hand is tedious.

**Scope**
- New print format **Spines**: a narrow vertical strip per item, coloured with the cover's dominant colour. The title (and optionally the creator) is rotated 90°, in black or white, whichever contrasts better with the colour.
- Dominant colour: computed from the image itself, with no new dependency.
- Spine height follows the cover height (or the size preset if item 1 is done). The width is fixed (for example 8–12 mm) or varies a little per item for a natural shelf look.

**Acceptance**
- Text is readable on light and dark colours, and long titles shrink or are truncated to fit.
- Tests for colour extraction (solid-colour fixtures) and for the contrast choice.

## 5. [x] TV series media type (311b6c7)

**Why:** TV trackers are as common as movie trackers. TMDB covers TV, so the existing API key can be reused.

**Scope**
- A new media type **TV series**, with input `Title, Year` (first air year optional).
- Sources: TMDB TV search (localized posters, like movies), with a no-key fallback if one is viable, such as the IMDb suggestion endpoint, which also returns series. Match on title and first air year.
- It appears in the media type selector, in the sample CSV download, in the TMDB key tip and in the CLI.

**Acceptance**
- Provider tests with recorded fixtures, as for movies. A well-known show downloads a poster with and without a TMDB key, where a fallback exists.
- README Sources section updated.

## 6. [x] Print-then-cut support (bleed and registration marks) (bbc4005)

**Why:** many users print onto sticker paper and cut with a Cricut or Silhouette machine, or want a small bleed so hand cutting leaves no white edges.

**Scope**
- An option **Cut mode**: **Border** (default, today's grey line), **Bleed** (each image extends 1 mm past its cut line, with crop marks), **None**.
- Optional registration marks in the page corners for cutting machines, with a note in the README on which machine settings apply.

**Acceptance**
- The default output doesn't change. With bleed on, images don't overlap their neighbours (the gap absorbs the bleed).
- Layout tests for bleed geometry.

## 7. [-] WONT DO: Grayscale / ink-saver mode

**Decision (2026-11-16):** not built. The printer's own grayscale or ink-saving setting gives the same result.

**Why:** many people print on cheap home inkjets, and some journals use a black-and-white aesthetic.

**Scope**
- A checkbox **Grayscale** in the Print sheet tab. Images are converted when the PDF is built; the downloaded files stay unchanged.
- Optional: a lighter "ink saver" variant (reduced contrast).

**Acceptance**
- The PDF images are grayscale; with the box unchecked, output doesn't change. Unit test on a colour fixture.

## 8. [x] Movie ticket stubs (664d66d)

**Why:** ticket stubs are a classic movie-tracker element.

**Scope**
- New print format **Ticket stubs**: a ticket shape with a perforation-style dashed line, a poster thumbnail, the title and year (from the file name), and empty "Date", "Where" and rating fields.
- It works for any media type but is labelled for movies first. Book items show the title and author.

**Acceptance**
- PDF export with several stubs per page and cut borders. Text fits as in review cards.
- Layout tests.

## 9. [x] Full-page cover grids ("Books of 2026") (b9ba95d)

**Why:** some users want a finished page to glue or bind into the journal, with no cutting.

**Scope**
- New print format **Full page**: a header the user types (for example "Books of 2026" or "Watched in March"), then the selected covers in a tight grid that fills the page. Cover size is chosen automatically so all covers fit on one page when possible.
- No cut borders; small rounded corners or a thin frame are optional.

**Acceptance**
- 1 to about 60 covers fit on one page; beyond that, it paginates with the header repeated.
- Tests for the auto-size calculation.

## 10. [x] TBR / watchlist checklist (2bfb272)

**Why:** to-be-read lists and watchlists are standard bujo collections.

**Scope**
- New print format **Checklist**: one row per item with a checkbox, a small thumbnail, the title and the creator. A header the user types (for example "TBR 2026").
- Rows paginate, and the thumbnail size fits the row height.

**Acceptance**
- PDF export for books and movies, with long titles truncated. Layout tests for rows per page.

## 11. [x] Polaroid frames (fde022c)

**Why:** instant-photo-style frames are a popular decoration style for media spreads.

**Scope**
- New print format **Polaroid**: each cover sits inside a white frame with a thicker bottom margin holding the title as a caption. Cut border around the frame.

**Acceptance**
- PDF export with the caption fitted to the frame. Layout tests.

## 12. [x] Music albums media type (with square covers) (43141c5)

**Why:** album trackers are a natural extension, and album art is square, unlike books and posters.

**Scope**
- A new media type **Albums**, with input `Album, Artist`.
- Source: iTunes Search API (`entity=album`, artwork scaled to a large size, as for Apple Books); consider a no-key fallback such as MusicBrainz plus the Cover Art Archive.
- Print sheet: square covers get square cells (the same cover height used as the width), instead of being shrunk into the 2:3 column. Mixed folders still lay out correctly.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with fixtures. A sheet of only square covers fills the page with square cells, and the 2:3 output doesn't change.
- README Sources and Print sheet sections updated.

## 13. [x] Video games media type (be6c483)

**Why:** game trackers exist in gaming bullet journals, but demand is less proven than for books, movies or TV.

**Scope**
- **Start the session by choosing a source**: compare candidates (for example IGDB, which needs a Twitch client ID and secret, or RAWG, which needs an API key) on terms of use, key requirements and cover quality. Present the choice to the user before implementing.
- A new media type **Games**, with input `Title, Year`, and the API key stored as for TMDB.
- Media type selector, sample CSV, key tip and CLI support.

**Acceptance**
- Provider tests with fixtures; the media type is skipped cleanly when no key is set. README Sources updated, with the key setup steps.

---

# Part 2: Beyond media

## Strong fit

There is a reliable image source, and the item fits how the app already works: a list in, images out, a print sheet.

## 14. [x] Museum artworks (cd132dc)

**Why:** museum-visit logs, "artwork of the month" pages and art history collections glue in mini prints of artworks. Museums sell "museum passport" journals built around this.

**Scope**
- A new media type **Artworks**, with input `Title, Artist`. This maps directly onto the existing title and creator fields.
- Sources, in order:
  1. Art Institute of Chicago API (no key, IIIF images).
  2. The Met Collection API (no key).
  3. Cleveland Museum of Art Open Access API (no key).
- **Decision (2026-11-17):** the Art Institute of Chicago is not used. Its image server answers every non-browser download with a Cloudflare challenge (HTTP 403). Sources are the Met, then Cleveland. Credits are stored as a `<file>.credit.txt` sidecar; the PDF credits page is left to item 15.
- Only public-domain / CC0 images are used. Use each API's own licence field and skip anything that isn't open access.
- Match on title and artist name. Prefer the work's primary image over detail shots.
- Artworks are often landscape: follow the **Non-2:3 images** convention.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with recorded fixtures. A famous public-domain painting downloads from at least one source, and a work that isn't open access is skipped with a clear reason.
- Landscape artworks print uncropped and correctly placed. The README Sources section lists the licence rules.

## 15. [x] Travel places (landmark photos and flags) (1cb6785)

**Why:** travel logs and "places visited" spreads use landmark pictures, postcard-style images and flags.

**Scope**
- A new media type **Places**, with input `Place, Country` (country optional).
- Landmark or city image: Wikidata search, then the image property (P18), then the file from Wikimedia Commons, downloaded at a print-friendly size. Reuse the existing Wikidata client code where it fits.
- Optional per-item **flag** mode: when the item is a country, download its flag. Use a public-domain flag source, such as the Commons flag file linked from Wikidata.
- Commons images have their own licences: follow the **Attribution** convention.
- Media type selector, sample CSV and CLI support.
- **Decision (2026-11-17):** flag mode is per item through the title (`Flag of Japan` or `Japan flag`) and needs a plausible country name match. Item 14's deferred PDF credits page is built here: a **Credits page** checkbox in the Print sheet tab (on by default) lists every image that has a credit sidecar.

**Acceptance**
- Provider tests with fixtures. A well-known landmark and a country flag both download, and their credit is stored.
- Landscape photos are placed correctly. The README explains the licence and credit handling.

## 16. [x] Visited countries map (cda7ca3)

**Why:** a world map with visited countries coloured in is one of the most common travel spreads, and drawing one by hand is hard.

**Done in branch `feat/visited-map`**, merged in cda7ca3 (see `odd/tasks/visited-map.md`):
- Map data: Natural Earth 1:50m admin-0 countries v5.1.2, simplified and quantized by `internal/sheet/mapgen` into the embedded `internal/sheet/mapdata/countries.bin` (147 KB), with its binary format and decoder (b509bb0); country matching by English or Spanish name, ISO alpha-2/alpha-3 code and aliases (817fd6f).
- Projections: Natural Earth for the world, Lambert azimuthal equal-area per continent, region views and far-ring culling (6269630).
- Page kind: `map` in `PageKinds` with filled and Colour-in drawing, small-country dots, the country list, localized labels and a default title (739b04d).
- Make pages form: Region, Visited countries, Colour swatches and List countries, in English and Spanish (937f902).
- Review fixes: decode validation tests, trailing data rejected, Encode limits matching Decode, a verified (SHA-256) Natural Earth download, list truncation tests, a list that never overflows its box, and a Go test that keeps the form's regions and colours in step with Go (1ad46dc..9ed3c90); regenerated bindings for the map labels (bca2cec).
- README Visited map section and the Natural Earth licence (the `docs:` commit after 9ed3c90).
- Decisions:
  - Scope change (user decision 2026-11-21): world and continent maps now; single-country maps (US states, provinces) are a follow-up. No Antarctica, no picking countries by clicking the map.
  - Kind id `map`, options `region` (`world` | `africa` | `asia` | `europe` | `north-america` | `south-america` | `oceania`), `countries` (names or codes), `colour` (`teal` | `coral` | `sage` | `lavender`), `list` (default on). Default title "Places I've been" / "Lugares que he visitado".
  - Unknown and ambiguous entries are errors that block export; the only ambiguous name is the Spanish "San Martín" (Saint Martin / Sint Maarten).
  - Natural Earth (public domain; names from Wikidata, CC0) at 1:50m keeps the small states; de facto borders as Natural Earth draws them. Douglas–Peucker 0.02°, quantized 0.001° (0.01° would erase Vatican and Monaco); output is deterministic and the download is pinned by SHA-256.
  - The world view stops at 58° S; a continent is only a view (neighbours in the box are drawn the same way), and rings more than 100° from its centre are skipped.
  - A visited country smaller than 1.5 mm both ways gets a dot. The list shrinks from 9 pt to 6 pt in 2–4 columns, then ellipsizes and ends with "+K"; the map keeps at least 60 % of the area.
  - The form mirrors Go's regions and colours by hand; `internal/sheet/mapcontract_test.go` reads `frontend/src/pageForms.ts` and fails on drift.

**Scope**
- A generated print format **Map**: the user picks or types the visited countries, and the app draws a world map with those countries filled. A page title is optional.
- Map data: a public-domain dataset (for example Natural Earth admin-0 countries, simplified), embedded in the binary. No network at print time.
- Options: colour, or outline only (for colouring by hand), and the page size if configurable.

**Acceptance**
- The PDF draws a clean vector map, and the selected countries are filled. Country name matching accepts common English names and ISO codes.
- Tests for name/ISO matching and for "selected countries are filled". The README notes the dataset's source and licence.

**Out of scope:** regional maps (US states, provinces). Note them as a follow-up.

## 17. [x] Vintage nature illustrations (9de3a30)

**Why:** bird-watching logs, plant and garden journals and nature journals use vintage natural-history plates. Collage sheets of them are very popular.

**Scope**
- A new media type **Nature**, with input `Name` (common or scientific, for example `Robin` or `Monstera deliciosa`).
- Sources: Biodiversity Heritage Library (check the API key requirement in the licensing step), with Wikimedia Commons as a fallback via the species' Wikidata entity. Prefer illustrations (plates) over photos.
- Rights are checked per item: only public-domain or clearly open images. Follow the **Attribution** convention, since BHL asks for credit with the contributing institution.
- Optional extension, in the same session if time allows: a **Plant care card** print format (illustration, name, watering, light and a note field).
- **Decision (2026-11-17):** Commons is the primary source (no key): Wikidata resolves the name to a taxon, then Commons finds public-domain or openly licensed plates (many are BHL scans). BHL is a fallback used only when the user sets a BHL API key; its WebP page images are saved as JPEG. BHL's name index is OCR-based (mostly text pages), so it came second. The Plant care card is left as a follow-up.

**Acceptance**
- Provider tests with fixtures. A common bird and a common houseplant each return an illustration, and their credit is stored.
- The README documents the rights check and the credit format.

## Medium fit

Useful, but either demand is less proven or the item adds a new kind of printable to the app.

## 18. [x] Moon phase calendar (82e96c5)

**Why:** moon phase trackers are a common bujo spread (no search result confirmed it). They need no network and no licensing.

**Done in branch `feat/moon-calendar`**, merged in 82e96c5 (see `odd/tasks/moon-calendar.md`):
- Phases: true new and full moons and the per-day phase in `internal/sheet/moon.go`, checked against the USNO 2024 table (86f7cc8).
- Page kind: `moon` in `PageKinds` with the Row, Grid and Year layouts, hemisphere mirroring, Colour-in outline moons, localized labels and a default title (2819df6); a current year outside 1900–2100 (a wrong clock) is an error (bf57b36).
- Make pages form: Month, Year, Layout and Hemisphere, in English and Spanish (2aa7b00); a partial year commit restores the last valid year, and the form's language listener can be disposed (c257874); order-independent tab tests (6164387).
- README Moon calendar section (the `docs:` commit after 6164387).
- Decisions:
  - Kind id `moon`, options `month` (1–12), `year` (1900–2100), `layout` (`row` | `grid` | `year`), `hemisphere` (`north` | `south`); a month or year of 0 means the current one, resolved in Go through an injectable clock. The form always sends explicit values.
  - Meeus, *Astronomical Algorithms* ch. 49 true phases, converted to UTC with the Espenak–Meeus ΔT polynomials (1900–2150); the drawn phase is the lunation fraction at local noon. All 25 USNO 2024 events within one minute. No new dependency.
  - Days and marks use the computer's local time zone: an event is marked on the local date it falls on.
  - Labels (month and weekday names, New/Full marks) reach Go as `PageSettings.Labels` (`json:"-"`), filled by the app from the UI language, like the Print covers labels.
  - The default title comes from an optional `defaultTitler` interface in `buildPage`: "<Month> <Year>", or "<Year>" for the Year layout.
  - The Year layout marks new and full moon with the mark's initial (N/F, in Spanish N/L).
  - Weeks start on Monday (a week start choice is item 28).
  - Form year: typing sends only a whole year in range; on commit a whole year out of range is clamped, a fraction rounded, and an empty or partial one (fewer than four digits) restored.

**Scope**
- A generated print format **Moon calendar**: one month or a whole year, drawn from a local astronomical calculation (phase per day, with new and full moon marked).
- Layouts: a month row (one moon per day), a month grid and a year overview.
- Options: year/month, filled or outline-only moons (for colouring), and the hemisphere (it flips how the lit side looks).

**Acceptance**
- Phase dates match a reference table, such as published new and full moon dates for a test year, within ±1 day. These checks are unit tests.
- The PDF renders all three layouts.

## 19. [x] Anime and manga media type (95235ea)

**Why:** anime and manga trackers are popular among younger bujo users. This is my assumption; no search result confirmed it.

**Scope**
- A new media type **Anime/Manga**, with input `Title, Year` and an anime/manga toggle, or two media types: decide in the session.
- Source: AniList GraphQL API (no key for public queries), large cover image. Match on romaji, English and native titles.
- Covers are copyrighted, the same situation as book covers today: personal use only, as the README already implies.
- Media type selector, sample CSV and CLI support.
- **Decision (2026-11-17):** two media types, **Anime** and **Manga**, served by one AniList provider. AniList's rate limit is currently 30 requests a minute, so the throttle spaces its requests 2 s apart.

**Acceptance**
- Provider tests with fixtures, and a well-known title downloads. Rate limits are handled through the existing throttle.

## 20. [x] Podcasts media type (75f9e9f)

**Why:** podcast trackers sit next to reading and watching logs. The iTunes Search API the app already uses returns podcast artwork.

**Scope**
- A new media type **Podcasts**, with input `Podcast, Host` (host optional).
- Source: iTunes Search API with `media=podcast`, artwork scaled to a large size. Reuse the Apple provider's helpers.
- Artwork is square: follow the **Non-2:3 images** convention.
- Media type selector, sample CSV and CLI support.
- **Decision (2026-11-23):** one source, the existing Apple provider with `media=podcast` (show title from `collectionName`, host from `artistName`, artwork at its original size, usually 3000 × 3000). No keyless fallback: the RSS feed is found through the same iTunes lookup, and Podcast Index needs a key. Registered last, as `podcasts`; item 12's square cells already place the artwork.

**Acceptance**
- Provider tests with fixtures. Square artwork prints in square cells.

## 21. [x] Photo crop to standard print sizes (daac236)

**Why:** people print their own photos onto sticker paper. The print sheet already takes any folder, but photos need fixed shapes.

**Scope**
- A **Crop to** option in the Print sheet tab: **None** (default, today's behaviour), **Instant mini** (54 × 86 mm frame, 46 × 62 mm image), **Square**, **Photo strip** (3 or 4 frames in one strip) and **Wallet** (64 × 89 mm).
- Centre crop by default, with optional face/centre heuristics left out of scope. A per-image crop offset is a stretch goal.
- The source files are never modified: cropping happens when the PDF is built.

**Acceptance**
- Cropped images fill their frames exactly, with no distortion. The default output doesn't change.
- Layout and crop math tests.

## Weak fit

There's a source or licensing problem, or the item isn't image-based. Kept for completeness; check the notes before choosing one.

## 22. [x] Recipe cards (878e701)

**Why:** recipe collections are a classic bujo spread. They include recipe cards, recipes-tried ratings and conversion charts.

**Weakness:** TheMealDB's free tier is for development and educational use, and its image licence is unclear. The better version doesn't download anything.

**Done in branch `feat/recipe-cards`**, merged in 878e701 (see `odd/tasks/recipe-cards.md`):
- Page kind: `recipes` (Recipe cards) with A6, 4 × 6 in and Full page sizes, fitted titles, shrinking body text, continuation cards, the photo, blank cards and the limits (b41e92e); its labels in the UI language (9934ed6).
- Markdown import: `ParseRecipesMarkdown` with English and Spanish headings and keys (1b34272); the `ImportRecipes` and `ChooseRecipePhoto` bindings (3914b7c).
- Page kind: `conversions` (Kitchen conversions) with the volume, weight, oven and cups-to-grams tables on every paper (a1dedf6); its labels in the UI language (e2cbaa7).
- Make pages forms: the recipe list and editor, Import…, the photo picker and Card size, and the conversions hint, in English and Spanish; review fixes to the Markdown import (bold keys, code fences, an empty list on cancel) (c8b5611, f93b599).
- Review fix: the conversions tables tolerate unresolved ingredient labels (ebbc048).
- README Recipe cards (with the Markdown format) and Kitchen conversions sections (the `docs:` commit after ebbc048). Feature document commits: 5614e96, 5ce01cf, 2263ca2, 7d0820a, 72e9f2a, fa22498.
- Decisions:
  - Source: none. TheMealDB was checked on 2026-11-23: its free key is for development or educational use, a public release needs a supporter (paid) key, and no licence is stated for its images. The roadmap rule (only if the images may be used) leaves it out; photos are the user's own files.
  - Import: Markdown only (user decision 2026-11-23); CSV is awkward for multi-line ingredient and step lists.
  - Kitchen conversions is its own page kind (user decision 2026-11-23), not a page of the recipe cards.
  - Kind ids `recipes` and `conversions`, registered after `map`. `recipes` options: `recipes` (title, servings, time, ingredients, steps, photo as an absolute path or `""`) and `size` (`a6` | `4x6` | `page`, default `a6`); it prints no shared title (each card has its recipe's title) and the cards have their own dashed cut lines, so the cut mode does not apply. `conversions` has no options (`{}`) and the default title "Kitchen conversions" / "Conversiones de cocina".
  - Sizes: cards are landscape, A6 148 × 105 mm and 4 × 6 in 152.4 × 101.6 mm, tiled within 8 mm margins 4 mm apart (2 a page on A4, Letter and B5); on A5, where neither fits, one card a page scaled down. Full page is one recipe a page in the content area, with no cut line.
  - Limits, else an error naming the recipe by number: 50 recipes; title 100 characters; servings and time 40; 60 ingredients of 200 and 40 steps of 600; photo path 4096 bytes; Markdown file 1 MiB. Control characters and space runs become one space; a recipe with no title, ingredients, steps or photo is dropped.
  - Flow: the body text shrinks from 9 pt (11 pt on Full page) by 0.5 pt until both columns fit the first card; at 6 pt the rest flows line by line onto continuation cards titled "Title (continued)", with no meta line or photo, the steps numbered on. Leftover slots on the last page, or a whole page with no recipes, are blank cards to fill in by hand.
  - Conversions: two by two tables on every paper; a US cup of 240 ml; ounces and pounds rounded to the gram; the standard gas mark table (°F = 250 + 25 × gas mark, °C rounded to 10); fractions written as 1/3 (the Go font has no ⅓); Spanish decimals with a comma; header fills left off with Colour-in.
  - Labels in the UI language through `PageLabels` (Ingredients, Steps, Serves, Time, continued, and the conversion chart words, the ten ingredients as one comma-separated key); unit symbols (ml, g, oz, lb, °C, °F) untranslated.
  - Out of scope: any recipe source, CSV import, a photo crop offset, recipe ratings, saving recipes between sessions.

**Scope**
- A generated print format **Recipe card**: the user enters a title, ingredients, steps, servings and time (typed in, or from a CSV/Markdown file), plus an optional photo of their own. The app lays it out as a card at common sizes (A6, 4 × 6 in, or bujo-page width).
- Optional companion page: a **kitchen conversions** chart (cups/grams/ml, oven temperatures), generated.
- TheMealDB lookup only if the licensing step confirms the images may be used. Otherwise, leave it out.

**Acceptance**
- Long ingredient lists flow to a second card, or shrink within limits. Text never overflows.
- Layout tests. The README shows the input format.

## 23. [x] Food and drink log images (Open Food Facts) (4d4506b)

**Why:** tea, coffee, wine, beer and snack logs glue in product or label pictures.

**Weakness:** Open Food Facts images are CC BY-SA, which requires credit and share-alike, and their quality varies a lot.

**Scope**
- A new media type **Products**, with input `Product, Brand`, or a barcode.
- Source: Open Food Facts API (front image). Follow the **Attribution** convention, and record the licence per image.
- Skip images below a minimum resolution.
- **Decision (2026-11-23):** name searches use Search-a-licious (`search.openfoodfacts.org`), because the legacy `/cgi/search.pl` was unavailable and the v2 API has no text search; a barcode (8 to 14 digits, no brand) is read through the v2 product API. The front image's shorter side must be at least 400 px. Credits go in the `<file>.credit.txt` sidecar (product and brand, contributor, source, CC BY-SA 3.0 licence, product page). Product reads are spaced 4.5 s apart and searches 6.5 s apart, under the published 15 and 10 per minute. Barcode lines save as `<barcode>.jpg`, since file names come from the input.

**Acceptance**
- Provider tests with fixtures. A common product returns a usable front image with its credit stored.

## 24. [x] Board games media type (e0382aa)

**Why:** board game trackers exist among hobby journalers.

**Weakness:** BoardGameGeek is the obvious source, but its API access rules may now require registration (not verified). Box art is copyrighted.

**Scope**
- **Start with the licensing/access step:** confirm BGG XML API access requirements, and present the findings and any key setup to the user before implementing.
- A new media type **Board games**, with input `Title, Year`. Source: the BGG search, then the thing image.
- Media type selector, sample CSV and CLI support. Store a token, if one is needed, like the TMDB key.
- **Decision (2026-11-23):** the BGG XML API2 needs a registered application (approved by hand) and its bearer token on every request. BGG asks apps for end users to ship the app's own token, so users need not register for one, so release builds embed one from the CI secret `BGG_TOKEN` (`-ldflags -X bujo-helper/internal/buildinfo.BGGToken=...`); a token in Settings, `-bgg-token` or `BGG_TOKEN` overrides it. Without a token the type is skipped. Requests are spaced 5 s apart. Credits go in the `<file>.credit.txt` sidecar (game and year, BoardGameGeek, game page, publisher copyright note). The "Powered by BGG" logo is shown in the Download tab (board games only) and in Settings → About.

**Acceptance**
- Provider tests with fixtures. The README documents the access setup.

## 25. [ ] Concerts

**Why:** concert logs are common, alongside ticket stubs.

**Weakness:** artist photos have unclear rights, and setlist.fm needs an API key. Ticket stubs are already item 8.

**Scope**
- A new media type **Concerts**, with input `Artist, Date, Venue`.
- Image: an artist photo from Wikidata (P18), then Commons, only with an open licence, following the **Attribution** convention.
- Optional: fetch the setlist from setlist.fm when the user provides a key, and print it on a card.

**Acceptance**
- Provider tests with fixtures. Without a key, the setlist is skipped cleanly. Credits are stored.

## 26. [ ] Sports teams (decision gate)

**Why:** sports fans track matches and seasons.

**Weakness:** team logos and crests are trademarks. **Not recommended.**

**Scope**
- **Start with a decision gate:** present the trademark concern to the user and implement only if they confirm.
- If confirmed: a media type **Teams**, with input `Team, Sport`, using Wikidata/Commons images only where Commons marks the logo as public domain (for example simple text logos). Everything else is skipped.

**Acceptance**
- Only images marked public domain are downloaded, and everything else is skipped with a clear reason.

## 27. [x] Quote cards (615bbab)

**Why:** quotes and lyrics pages are common bujo collections.

**Weakness:** there are no images to fetch, and free versions are everywhere, so the app adds little.

**Done in branch `feat/quote-cards`**, merged in 615bbab (see `odd/tasks/quote-cards.md`):
- User extension (2026-11-21): search an author or personality and tick their quotes, instead of only typing or importing them. That is what makes the kind worth having over the free templates.
- Wikiquote search: `internal/quotes` (MediaWiki full-text search, page HTML parsing, section filter, text cleaning, limits) with recorded en/es fixtures (8c95a68, fixtures trimmed in 99ff3b7); the `SearchQuotes` binding in the UI language (68fe75a); a best page without quotes falls back to the next matches and keeps the other matches (7404937), and a failed fallback keeps the best page (eecc949); race-free test request logs (92f7a54).
- Page kind: `quotes` in `PageKinds` with the Classic, Bold and Framed styles, 4/6/8 cards a page, fitted text, empty card frames and the Wikiquote credit (852d0e7); its title and credit in the UI language (d23384c).
- Make pages form: Find quotes by, Other matches, the results checklist, Picked quotes across searches, Your own quotes, Style and Cards per page, in English and Spanish (82253ba).
- README Quote cards section with the Wikiquote licence and what the search sends (the `docs:` commit after 07f31e2).
- Decisions:
  - Source: Wikiquote (en and es) through the MediaWiki Action API, no key, the contact User-Agent, one request at a time. Quotable (unreliable), ZenQuotes and API Ninjas (author search needs a paid key) and Wikidata P1683 (too sparse) were rejected.
  - Licence: Wikiquote text is CC BY-SA 4.0; each page with a Wikiquote quote prints "Quotes from Wikiquote (CC BY-SA 4.0)". Quotes from copyrighted works are on Wikiquote as fair use; the README says to print a few for personal use. The user picks single quotes; the app never prints a page's whole list.
  - Kind id `quotes`, registered after `habits` and before `map`; options `quotes` (text, author, source `""` or `wikiquote`), `style` (`classic` | `bold` | `framed`), `perPage` (4 | 6 | 8, default 6). At most 200 quotes, 600 characters of text and 100 of author, else an error; control characters and space runs become one space; a quote with no text is dropped. Default title "Words to keep" / "Palabras para guardar". The cards have their own dashed cut lines, so the cut mode does not apply.
  - Quotes per page: top-level list items outside Disputed, Misattributed, Attributed, Quotes about, See also, References and similar sections (and the Spanish equivalents), 10–400 characters, deduplicated, at most 60. A search reads at most three pages.
  - Text starts at 22/18/15 pt for 4/6/8 cards (Bold a fifth larger), shrinks by 0.5 pt to 8 pt, then ends with "…"; hollow with Colour-in at 14 pt or larger. Author in the Go font, one line. The credit band is always reserved, so card sizes never depend on the source.
  - Typed quotes split at the last comma (`Quote, Author`); a field in double quotes may hold commas (`Free at last, "Martin Luther King, Jr."`, added in review follow-ups 2); picks keep their order across searches, deduplicated by text and author; the form sends the picks, then the typed quotes.
  - Out of scope: CSV or Markdown import, other quote sources, lyrics, keeping search results between sessions.

**Scope**
- A generated print format **Quote cards**: the user types or imports quotes (`Quote, Author`), and the app lays them out as cards in a few typographic styles. It uses the lettering fonts from item 31 instead of adding new ones.

**Acceptance**
- Text auto-fits each card without overflow. Layout tests.

## 28. [x] Habit trackers and calendars (e2502a4)

**Why:** monthly calendars and habit trackers are core bujo pages.

**Weakness:** they are generic, free templates are everywhere, and they aren't image-based.

**Done in branch `feat/calendars`**, merged in e2502a4 (see `odd/tasks/calendars.md`):
- Calendar math: days in month (leap years) and a month's week rows for a week start in `internal/sheet/calendar.go` (a57b5fa).
- Page kinds: `calendar` (Month calendar) with Monday or Sunday week starts (7d8ffdb) and `habits` (Habit tracker) with paginated rows, blank rows when empty and a default title (d7a6b6c); the page count follows the paper through an optional `paperPager.pagesOn`.
- Make pages forms: Month and Year fields shared with the moon form (88bf494); Week starts on, and Habits one per line with a limits hint, in English and Spanish (b9afcc9).
- Review fixes: habit names with control characters are normalised, and the habit tracker's A4 page count is never below 1 (7750464).
- README Month calendar and Habit tracker sections (the `docs:` commit after 7750464).
- Decisions:
  - Kind ids `calendar` and `habits`, registered after `moon` and before `map`. Options `month` (1–12), `year` (1900–2100), plus `weekStart` (`mon` | `sun`) or `habits` (a list); a month or year of 0 means the current one, resolved in Go through the injectable clock the moon uses.
  - At most 40 habits of 60 characters, else an error; the form sends what was typed and Go's error shows in the preview. Each run of control characters or spaces in a habit becomes one space, then the name is trimmed; a habit of only such characters is dropped like a blank.
  - Default titles: "<Month> <Year>", and "Habits · <Month>" ("Hábitos · <Mes>"); the label is substituted with `strings.ReplaceAll`, never a format string. Titles use the lettering font; weekday names, day numbers and habit names stay in the Go font.
  - Calendar cells are at most 1.5 times as tall as wide, and the grid is centred; empty leading and trailing cells have no box.
  - Habit tracker: the name column fits the longest name between 20 mm and 30 % of the width; boxes are square, at most 6 mm; rows are 1.5 boxes high, at least 6 mm; rows that do not fit go on further pages under the repeated title.
  - Out of scope: weekly spreads, a yearly calendar, days as rows, holidays.

**Scope**
- Generated print formats **Month calendar** (any month/year; week start Monday or Sunday) and **Habit tracker** (a habit list × the days of the month grid).
- Sized to fit common bujo pages (A5, B5) and the page sizes the app supports.

**Acceptance**
- Calendar dates are correct for leap years and for both week starts (unit tests). The PDF renders both formats.

---

# Part 3: App structure and lettering

The generated printables (items 16, 18, 27, 28) don't fit the Print sheet tab. That tab is built around a folder of images: `sheet.build`, `ExportPDF` and `PrintPages` all fail without image paths, export is disabled with nothing selected, and the only visual is the thumbnail gallery. These items give generated pages a home of their own and add lettering, which several of them need.

Order: 29 → 30 → 31, then the generated printables.

## 29. [x] Make pages section and app navigation (e7f8b6f)

**Why:** generated pages need typed options and no folder. Forcing them under the Print sheet **Format** control would leave half its controls (folder, gallery, selection, cover size, crop) meaningless for them. The Print options are also one long flex row of nine controls, and adding font, colour-in and page-specific fields would make it unreadable.

**Already done** (layout restyle, branch `feat/ui-restyle`; see `odd/tasks/ui-restyle.md`):
- Tabs: the tab list is data-driven (read from the markup), "Print sheet" is renamed **Print covers** / **Imprimir portadas** with the internal id `print` kept, and the `#app[data-tab="print"] #media-type` rule is gone (12513cb).
- Download tab layout: the media type is a grid of radio buttons (5 per row) inside the panel, and the tab has two columns: input left, progress + results + footer right (6c92f1f).
- Print covers layout: a 300 px options column with **Covers**, **Layout**, **Page** and **Lettering** fieldsets, a fieldset with no visible field is hidden, and the gallery has 6 cards per row with a round check badge and an accent border when selected (20e2c1b, 5bd1c5a).
- EN and ES strings for the renamed tab and the fieldset legends.

**Done in branch `feat/make-pages`**, merged in e7f8b6f (see `odd/tasks/make-pages.md`):
- **B5** paper (ca1b8bf).
- Download: the sources-tried line under the media types (`MediaTypeInfo.Sources`, provider display names through an optional `Label()`), and the **Print these covers** link, which opens Print covers on the finished job's folder (d45c5b6, b1353f8).
- No-image backend: a page kinds registry in `internal/sheet/pages.go` (each kind decodes its own typed options from JSON, unknown fields rejected), the **Dot grid** kind (4 / 5 / 6 mm, optional title), and the `PageKinds`, `PagePages` and `ExportPage` bindings, with options passed as a JSON string so new kinds don't change them (168e2cd).
- **Make pages** tab: page kinds list, kind form registry (`pageForms.ts`), shared **Page** and **Lettering** fieldsets (`fieldsets.ts`) rendered by both tabs, Paper as a segmented group, the muted Lettering hint in Print covers, and a page-shaped preview placeholder with the page count (b1353f8, 71fb1a2).
- Decisions: the dot grid title sits in the Lettering fieldset (the shared title/header text), not in the kind form; selected segments use the accent-soft fill.

**Scope**
- **Three tabs:** **Download**, **Print covers** (today's Print sheet, renamed) and **Make pages**. Make the tab list data-driven instead of the two hard-coded entries in `main.ts`, and fix the `#app[data-tab="print"]` CSS rules.
- **Download tab** (canvas: Download): two columns.
  - **Left column, the input:**
    - The media type moves out of the global header and becomes a **grid of buttons** (5 per row; the selected one has an accent border and a tinted background). Below it, a muted line lists the sources tried for that type, in order, from the catalog's `Providers`.
    - Import CSV… / Sample CSV, the item count, the items textarea, Output folder + Country, and **Download** + Cancel.
  - **Right column, the results:** the progress bar and the results list. The footer has **Open folder** and a **Print these covers** link that switches to Print covers.
- **Print covers layout** (canvas: Print covers): an options column on the left (about 300 px), grouped in `<fieldset>`s, with toolbar + gallery on the right:
  ```
  ┌ Covers ──────────┐  ┌───────────────────────────────────────┐
  │ Folder [….][Brw] │  │ [Gallery|Preview]   all · none  8 sel │
  ├ Layout ──────────┤  │ ┌──┐┌──┐┌──┐┌──┐┌──┐┌──┐               │
  │ Format  [▾]      │  │ │ ✓││  ││ ✓││  ││ ✓││  │  6 per row    │
  │ Size [▾] Crop[▾] │  │ └──┘└──┘└──┘└──┘└──┘└──┘               │
  │ Cards for [▾]    │  │                                        │
  ├ Page ────────────┤  │                                        │
  │ Paper[▾] Cut[▾]  │  │                                        │
  │ [✓] Credits page │  │                                        │
  ├ Lettering ───────┤  └───────────────────────────────────────┘
  │ Header [……]      │  status            [Open PDF] [Export PDF…]
  │ Font[▾] □Colour  │
  └──────────────────┘
  ```
  - **Shared fieldsets:** **Page** and **Lettering** are shared components that both tabs render.
  - **Show/hide rules:** keep the existing ones (`updateFormat`); they now hide fields inside fieldsets. A fieldset with no visible fields is hidden.
    - The Lettering fieldset only shows for formats that print a header (Full page, Checklist). Otherwise a muted hint says so.
    - The Gallery | Preview switch is added by item 30, and the Font and Colour-in fields by item 31.
  - **Gallery cards:** each card shows the cover thumbnail with a round check badge in its top-right corner and the file name below. A selected card gets an accent border.
- **Make pages tab** (canvas: Make pages): three columns. On the left, a list of page kinds (about 200 px). In the middle, an options column (about 320 px) with the kind's form and then the shared Page and Lettering fieldsets. On the right, the preview with the status line and export buttons below it:
  ```
  ┌─────────────┬──────────────────────┬──────────────────────────┐
  │ PAGE        │ Moon calendar        │ Preview      Page 1 of 1 │
  │▸Dot grid  29│ Month [▾]  Year [  ] │ ┌──────────────────────┐ │
  │ Moon cal. 18│ Layout [Row|Grid|Yr] │ │   January moons      │ │
  │ Month cal.28│ Hemisphere [N|S]     │ │  ◐ ◑ ○ ● …           │ │
  │ Habits    28│ ┌ Page ────────────┐ │ │                      │ │
  │ Quotes    27│ │[A5|B5|A4|Letter] │ │ │                      │ │
  │ Map       16│ │ Cut [▾]          │ │ └──────────────────────┘ │
  │             │ ├ Lettering ───────┤ │ status                   │
  │             │ │ Font [Pacifico▾] │ │  [Open PDF] [Export PDF…]│
  │             │ │ □ Colour-in      │ │                          │
  │             │ └──────────────────┘ │                          │
  └─────────────┴──────────────────────┴──────────────────────────┘
  ```
  - **Page kinds list:** kinds that aren't built yet are not listed. The list comes from Go, like `PrintFormats()`.
  - **Option controls:** options with two to four values use segmented button groups (Layout, Hemisphere, Paper, Dot spacing, Week start). Longer lists use selects.
  - **Preview:** a page-shaped placeholder (the paper's aspect ratio on a grey well) until item 30 makes it live.
- **Backend:** a no-image path next to the image one, for example `PageKinds()`, `PagePages(kind, opts)` and `ExportPage(kind, opts)` bindings, plus a builder in `internal/sheet` (or a new `internal/pages`) that takes typed options instead of paths. It reuses `newPDF`, papers, cut modes and the text helpers. Each kind declares its own options struct, decoded from JSON, so later items add kinds without touching the bindings.
- **First kind: Dot grid page.** A plain dot grid (5 mm pitch, optional title) proves the path end to end. Dot grid paper is the basic bujo page, so it's useful on its own.
- Paper sizes: add **B5** (176 × 250 mm), which bujo notebooks use and item 28 asks for.
- EN and ES strings for every new label.

**Acceptance**
- Tabs work with keyboard navigation, as today. The Download tab still works, and its media type buttons are inside it and reachable by keyboard.
- Print covers output is byte-identical to before for the same options (compare page-stream hashes, as in earlier items).
- Make pages exports a dot grid PDF on every paper size. Go tests cover the dot positions and margins, and the frontend tests cover tab switching and fieldset visibility.

**Out of scope:** the preview pane stays a placeholder until item 30. The Lettering fieldset only holds today's Header field until item 31.

## 30. [x] Live PDF preview (5439969)

**Why:** generated pages have no thumbnails, so without a preview the user exports blind. A real preview also helps Print covers (spines, stubs and polaroids are hard to imagine from the gallery).

**Done in branch `feat/live-preview`**, merged in 5439969 (see `odd/tasks/live-preview.md`):
- Backend: `internal/preview` keeps the last 4 renders in memory and serves them at `/preview?id=N` next to `/thumbs`; the `PreviewPDF` and `PreviewPage` bindings render through the same functions as `ExportPDF` and `ExportPage` and answer the URL and page count (9657cae).
- **Make pages** shows the live preview by default, with ‹ › and "Page X of N"; requests are debounced (300 ms) and numbered so only the newest is drawn (e72af35).
- **Print covers** gets a **Gallery | Preview** switch; Preview shows the selected covers with the export's options, and renders again after a language change (stub, card and credits labels) (65f18dd).
- Review follow-up: a page that cannot be drawn shows the error, and the old page stays, dimmed, until the next one is drawn (9ece906). README documents the preview and the pdf.js licence (the `docs:` commit after 9ece906).
- Decisions:
  - pdf.js on a `<canvas>` on every platform: WebKitGTK's built-in PDF.js viewer is behind a setting Wails doesn't expose (and older builds lack it), and the native viewers show their own chrome.
  - The **legacy** pdf.js build: the modern one calls `Map.prototype.getOrInsertComputed` and `Math.sumPrecise` unguarded, which only the newest engines have.
  - The frontend fetches the PDF bytes and passes them to `getDocument({data})`, because pdf.js streams only http(s) URLs and Linux serves the app from `wails://`.
  - fpdf stamps `CreationDate` with the current time, so the tests that compare preview and export bytes pin the dates and set the catalog sort.
  - On an option change the preview stays on the page shown, clamped to the new page count.

**Scope**
- Go renders the current options to PDF bytes in memory and serves them through the existing asset handler (the one behind `thumbUrl`). No temp file is left behind.
- The frontend draws page 1, with next/previous for more pages, onto a `<canvas>` using **pdf.js** (`pdfjs-dist`, Apache 2.0, bundled by Vite and not loaded from a CDN). Verify first whether WebKitGTK, WebView2 and WKWebView render PDFs natively in an `<iframe>`. Assumption: WebKitGTK does not, so pdf.js is needed on Linux.
- Debounce re-renders (about 300 ms after the last option change) and drop stale renders.
- Make pages shows the preview by default. Print covers gets a **Gallery | Preview** switch on the right-hand pane.

**Acceptance**
- The preview matches the exported PDF (same bytes for the same options). Changing an option updates it within about a second for a one-page document. The licence for pdf.js is recorded in the README.

## 31. [x] Lettering fonts and hollow letters (d492a1d)

**Why:** hand lettering is central to bujo headers, and printed headers in a plain sans font look out of place on a hand-drawn spread. Hollow (outline-only) letters let the user colour headers by hand, the same way they colour bubble lettering drawn with a pen.

**Done in branch `feat/lettering-fonts`**, merged in d492a1d (see `odd/tasks/lettering-fonts.md`):
- Registry and delivery: ten fonts embedded in `internal/sheet/fonts/<id>/` with their licences, a registry (`LetteringFonts`, `LetteringFont`, `LetteringFontByID`), the `LetteringFonts()` binding, and `internal/fonts` serving `/fonts/<id>.ttf` and `/fonts/<id>/licence` next to `/thumbs` and `/preview` (6266d44); a content-hash ETag so cached fonts revalidate after an upgrade, and the id charset asserted (64c77bb).
- PDF: `Font` and `ColourIn` on `Options` and `PageSettings`; Full page and Checklist headers, the Dot grid title and Review card titles draw in the chosen font, hollow with Colour-in; the credits page gains a Fonts section (867dd0a).
- UI: the font picker listbox and Colour-in controls (7ce9e03), wired into the Lettering fieldset of Print covers and Make pages (b22d7e7).
- Docs and About: Settings → About lists every bundled font with its licence, read in place (1c41692); tests for font load failures and the served bytes (bcaab2c); README Lettering section, font table and third-party licences (the `docs:` commit after 1c41692).
- Decisions:
  - Font ids `go` (default; empty means `go`), `kaushan-script`, `pacifico`, `sacramento`, `patrick-hand`, `amatic-sc`, `titan-one`, `bungee`, `luckiest-guy`, `bungee-outline`, `londrina-outline`; groups `sans`, `brush`, `monoline`, `handwritten`, `display`, `hollow`; hollow capability `stroke`, `none` (Sacramento, Amatic SC) or `native` (Bungee Outline, Londrina Outline).
  - The credits page lists the bundled font only when that page prints anyway (Credits on and at least one image credit): a font never adds a page, so "no credit file, no page" and `CreditPages` stay as they were. Page kinds have no credits page; the README and Settings → About list every font.
  - A `usesLettering` format flag (Full page, Checklist, Review cards) shows Font and Colour-in in Print covers; the Header text stays tied to `usesHeader`. Make pages always shows them.
  - Bundled fonts fit headers into 96 % of the width (swash margin); the Go font keeps 100 % and its bold face, so default output is byte-identical (2700 PDFs compared against 782ab17).
  - Outline width max(0.2, 0.25 × pt / 20) mm, black, only at 14 pt or larger; mode 0, line width and draw colour are always restored.
  - A `native` font shows Colour-in checked without overwriting the stored preference, which is sent to Go (Go never strokes those fonts) and comes back with the next font.
  - Review card titles are 9.5 pt, too small for outlines, so they always print filled.
  - Fonts reach the webview through the asset handler, not the Vite bundle.

**Research (2026-11-20)**
- **Licences.** SIL OFL 1.1 allows bundling fonts with software, provided each copy ships the copyright notice and licence text (clause 2, [official text](https://openfontlicense.org/open-font-license-official-text/)). The OFL doesn't apply to documents made with the font, and embedding subsets in PDFs is fine ([OFL FAQ 1.12–1.13, 1.20](https://openfontlicense.org/ofl-faq/)). Apache 2.0 fonts need the licence text shipped. Ship the original TTFs unmodified. Pre-subsetting a font and keeping its name would clash with Reserved Font Names (Kaushan Script, Sacramento, Monoton, Londrina, Titan). fpdf subsets inside each PDF anyway.
- **fpdf v0.9.0 support** (checked in source and a probe PDF):
  - `AddUTF8FontFromBytes` embeds TTFs from memory, as `internal/sheet/cards.go` already does for the Go fonts. Only static `.ttf` with `glyf` outlines works: CFF `.otf` and `.ttc` are rejected, and variable fonts render only their default instance.
  - Subsetting is automatic. 21 fonts in one probe PDF came to 181 KB.
  - `SetTextRenderingMode(1)` emits the PDF `Tr` operator for stroke-only text. Stroke colour comes from `SetDrawColor` and width from `SetLineWidth`. fpdf doesn't track the mode, so it must be reset to 0 after each use.
  - There is no OpenType shaping or kerning (no GSUB/GPOS). Script fonts lose contextual alternates. Pacifico and Dancing Script still connect acceptably, and Caveat loses its letter variation.
  - **go-pdf/fpdf is archived** (v0.9.0 is the last release), so shaping, OTF and variable-font support will never come.
- **How hollow mode looks** (probe): bold display fonts (Bungee, Titan One, Luckiest Guy, Pacifico, Patrick Hand) make good hollow letters. Thin and monoline fonts (Sacramento, Amatic SC, Caveat) turn into hairline double strokes with no room to colour. Fonts that are already outlined (Bungee Outline, Londrina Outline) are hollow by design, and stroking them doubles every line.

**Scope**
- **Ten bundled fonts** (about 1.6 MB raw), embedded with `go:embed` under `internal/sheet/fonts/<id>/` together with each font's `OFL.txt` or `LICENSE`:

  | Style | Fonts | Hollow mode |
  |---|---|---|
  | Default | Go Regular/Bold (already shipped) | stroke |
  | Brush script | Kaushan Script, Pacifico | stroke |
  | Monoline script | Sacramento | not offered (too thin) |
  | Handwritten print | Patrick Hand, Amatic SC | Patrick Hand: stroke; Amatic SC: not offered |
  | Bold display ("bubble") | Titan One, Bungee, Luckiest Guy | stroke |
  | Hollow by design | Bungee Outline, Londrina Outline | already hollow; Colour-in is always on |

  All of them cover the Spanish letters (á é í ó ú ñ ü ¿ ¡). Luckiest Guy and Londrina Outline lack Latin Extended (ő ł č); note it for future locales.
- **A font registry** in Go: id, label, style group, TTF bytes, licence, and a hollow capability (`stroke`, `none` or `native`). It is exposed to the frontend like `PrintFormats()`.
- **Lettering fieldset** (canvas: Lettering picker, and the Make pages font popover):
  - **Font picker:** a button showing the current font's name in that font, with a chevron. It opens a listbox (`role="listbox"`, keyboard navigable). Native `<select>` options can't show custom fonts in WebKitGTK.
    - Each row shows the font name in its own face at about 21 px, its style group, and a badge: **outline ok** (`stroke`), **hollow** (`native`) or **no outline** (`none`).
    - Serve the same embedded TTFs to the webview through the asset handler for `@font-face`, so the fonts aren't duplicated in the frontend bundle.
  - **Colour-in checkbox**, with a one-line muted hint below it.
    - It stays enabled for every font, because it also outlines moons and the map.
    - With a `none` font, the hint says the letters stay filled.
    - With a `native` font, it shows as checked for letters, and the hint says the font is hollow by design.
  - the existing **Header** text, when the format uses it.
- **Applies to display text only:** full page and checklist headers, review card titles, and later month names, page titles and quote text. Captions, dates, stub text and credits stay in the Go font.
- **Hollow drawing:** a helper that sets mode 1, a stroke width of about 0.25 mm (scaled with the font size, minimum 0.2 mm) and draw colour black, draws, then always restores mode 0. Hollow text is only used at 14 pt or larger. Below that, it falls back to filled text.
- **Text fitting:** the existing `fitHeader`, `fitLines` and `GetStringWidth` code must use the chosen font. Script fonts with swashes need a small safety margin.
- **Credits:** the credits page and the README list the bundled fonts with their licences. The About text in Settings links the licence files.

**Acceptance**
- A test loads every bundled font through fpdf and fails if `pdf.Err()` is set. Another test checks that each font covers the EN and ES alphabets.
- A hollow-mode test checks that the content stream contains `1 Tr` … `0 Tr` and that no `Tr` state leaks to the next text or the next page.
- With the default font and Colour-in off, every existing format is byte-identical to before.
- Header text auto-fits without overflow in every font (layout tests, table-driven over fonts).

**Out of scope:** user-supplied fonts (a later item could allow importing a TTF, with the same `glyf`-only check), OpenType shaping, and outline versions of cover images. Turning photos into line art for colouring is a different, much harder feature; note it as a separate idea.

---

# Part 4: Follow-ups and possible features

Open items found during the reviews of Parts 1–3 (see the feature documents in `odd/tasks/`). They are possible features, not commitments; ordered by value first, then effort. Review fixes for the thumbnail tests and follow-up items 5–8 (frontend test gaps, CirrusSearch escaping, preview resize replay, quote comma round-trip) were done separately in `odd/tasks/review-followups-2.md` and are not listed here.

## 32. [ ] Preview cache for large cover selections

**Why:** re-rendering the live preview of a large cover selection re-encodes every image each time an option changes (source: `odd/tasks/live-preview.md`).

**Scope**
- Cache the decoded/encoded image data per path and file modification time between preview renders, bounded in memory, and drop entries when files change or leave the selection.
- Reuse the cache for export only if output bytes stay identical.

**Acceptance**
- A benchmark or test shows a second render of the same selection does not re-encode unchanged images. Export output is byte-identical with and without the cache. Memory stays bounded (test with a lowered cap).

## 33. [x] Plant care card page kind (541f295)

**Why:** the optional extension of item 17 (Vintage nature illustrations): a card next to a plant plate with watering, light and notes.

**Scope**
- A Make pages kind (or a Print covers format for nature images) with a plant name, plate image, and ruled fields (light, water, soil, notes), following the **Lettering** and **Colour-in** conventions.

**Acceptance**
- Layout tests per paper; text fits; EN/ES labels.

**Done in branch `feat/plant-care-cards`**, merged in 541f295 (see `odd/tasks/plant-care-cards.md`):
- Page kind: `plants` (Plant care cards) in the recipe card sizes, the plate contained (not cropped) at the top, the name in the lettering (hollow with Colour-in at 14 pt or larger) or read from the image's file name, ruled Light/Water/Soil/Notes fields in the UI language, blank cards; the `ChoosePlantImage` binding (e5382d5).
- Make pages form: the plant list and editor, the image picker and Card size, in English and Spanish, sharing the card list with the recipe form; README Plant care cards section (cd2cddd).
- Decision: a Make pages kind, not a Print covers format (formats cannot be limited to a media type; recipe cards already give the card model).

## 34. [ ] Regional maps for the visited map

**Why:** single-country maps (US states, Spanish provinces, Canadian provinces) are common travel spreads; left out of item 16.

**Scope**
- Add admin-1 regions from Natural Earth for a chosen set of countries, a region picker, and matching by region name. Same drawing, list and Colour-in rules as item 16.
- Keep the embedded data size in check (simplify geometry; only the chosen countries).

**Acceptance**
- Matching tests per country; layout tests per paper; README lists the data source and licence.

## 35. [ ] Colour-in for Review cards titles

**Why:** Colour-in has no visible effect on Review cards: titles print at 9.5 pt, below the 14 pt minimum for hollow text (source: `odd/tasks/lettering-fonts.md`).

**Scope**
- Decide (with the user) between a larger hollow title band on review cards or hiding the Colour-in control for that format; then implement it.

**Acceptance**
- Either hollow titles show on review cards (content stream has `1 Tr`), or the control is hidden for them with a hint. Default output unchanged.

## 36. [ ] Full page with square cells for albums

**Why:** album covers are square; the full page format still shrinks them into 2:3 cells (source: `odd/tasks/albums.md`).

**Scope**
- Full page uses square cells when the selection is mostly square, reusing the non-2:3 placement from item 14.

**Acceptance**
- Layout tests with square, 2:3 and mixed selections; other formats unchanged.

## 37. [ ] Cut option for review cards and spines

**Why:** Print-then-cut (item 6) applies to the cover sheet only; review cards and book spines keep their borders and ignore the cut option (source: `odd/tasks/print-then-cut.md`).

**Scope**
- Apply the cut modes (border, bleed + registration marks, none) to review cards and spines.

**Acceptance**
- PDF tests check bleed placement and marks for both formats; default output byte-identical.

## 38. [ ] Product barcode file names and contact User-Agent

**Why:** barcode downloads are named after the barcode, not the product; and the Open Food Facts contact User-Agent has no email (source: `odd/tasks/products.md`).

**Scope**
- Name barcode downloads `Product - Brand.jpg` when the lookup returns a name.
- Decide with the user whether the User-Agent carries a contact email (privacy trade-off) and document it.

**Acceptance**
- Downloader tests for barcode naming and duplicate suffixes; README updated.

## 39. [x] Test and robustness polish (0585dd0)

**Why:** small review advisories left open across features; each is cheap, together they harden the code.

**Scope**
- Live preview: busy indicator cleared early on next/previous while a new document's first page draws; tests for a failing `loadingTask.destroy` and for `holdDraw()` rebinding (`frontend/src/preview.ts`, `previewMock.ts`).
- Print formats: PDF content-stream checks for spine colour and rotation, card text and bleed placement; spines' colour buckets can split a hue (README overstates); bleed pad uses vertical px/mm; a failed `PrintFormats` binding also disables the cover sheet export; ticket stubs advisories (`stubs.go:120-129`, `pdf_test.go:728`).
- Lettering: body check on the Go licence route (`app_test.go`), and the retry test in `letteringLoad.test.ts` relies on one microtask hop.
- Photo crop: the `printCroppedImage` warning (`internal/sheet/crop.go:186-192`) and `anilist_test.go:186` suggestion; cover-size error text assertion.
- Polaroid: A5 Standard's 3 columns depend on `newFixedGrid`'s tolerance (pin it if the tolerance changes).

**Acceptance**
- Each fix has a test that fails before it; no output change except where a bug is fixed.

**Done in branch `feat/test-robustness-polish`** (see `odd/tasks/test-robustness-polish.md`):
- Frontend: a failing `loadingTask.destroy` (throw or rejection) no longer leaks an unhandled rejection; `holdDraw()` keeps every held draw; a failed `PrintFormats` binding falls back to the built-in covers format so the cover sheet still exports; the lettering retry test waits for the fonts instead of one microtask hop (865cf63).
- Print formats: bleed is padded from both axes, so the horizontal bleed no longer falls short; content-stream tests for spines, card text, bleed placement and ticket stub notches and perforations; README spines colour wording corrected (934d956).
- Tests: the Go licence route body, AniList title matching per media type and independent of AniList's order, and `newFixedGrid`'s exact-fit tolerance.
- The busy indicator and cover-size error text were already fixed and tested before this item.

## 40. [x] Release and build tooling

**Why:** leftovers from the release work (sources: `odd/tasks/bujo-helper-release.md`, `odd/tasks/boardgames.md`, `odd/tasks/live-preview.md`).

**Scope**
- Release workflow: warn when the `BGG_TOKEN` secret is empty; check after the build that the `-X` token symbol was linked; pass `ref_name` through env in `run` lines; remove the duplicated build step; stop hard-coding `productVersion` in `wails.json`.
- Frontend: split the 570 kB main chunk (`manualChunks`, e.g. pdf.js) so `vite build` stops warning; type-check test files; i18n leftovers (split sentence fragments, tooltip text in both markup and dictionary, keys built from `undefined`); move the listener test cleanup into `afterEach`; label fit test across poster ratios.

**Acceptance**
- `vite build` has no chunk warning; the workflow passes `actionlint` (or a YAML parse check); tests for the i18n fixes.

**Done in branch `feat/release-build-tooling`** (see `odd/tasks/release-build-tooling.md`):
- Release workflow: one bash build step for all three OSes, built with `-trimpath`; a warning when `BGG_TOKEN` is empty, and a failed build when the token is not in the binary; the tag and asset names go through env; the version comes from the tag (`v1.2.3[-suffix]` → `1.2.3`, other tags fail) and `wails.json` keeps `0.0.0` (6ea15f3, 8fe290c).
- Frontend: pdf.js in its own chunk, so `vite build` no longer warns (3ec2283); test files type-checked in `npm test` through `tsconfig.test.json` (26e0a50); key tips as whole sentences, tooltips only in the dictionary, no keys built from `undefined`, listener test cleanup in `afterEach` (c9a536f).
- Label fit tested across six poster ratios, with a long label that gets truncated (712384d).
- Checked with a YAML parse; `actionlint` is not installed.

## 41. [ ] Source checks and documented limitations

**Why:** checks that need an external step, and limitations kept as documented behaviour (sources: `odd/tasks/nature.md`, `odd/tasks/places.md`).

**Scope**
- Run a live BHL check once a key exists (BHL often marks public-domain scans CC BY-NC, which the rights rule skips); record the result in the README.
- Places: a place name ending in "flag" switches to flag mode; odd Commons author fields ("FeaturedPics", "No machine-readable author provided") are stored as given. Decide whether to clean them up or keep them documented.

**Acceptance**
- README records the live check result; any clean-up has fixture tests.
