# Roadmap: bullet journal printables

New printables and content types for bullet journal users, based on research into common bujo spreads.

- **Part 1 (items 1–13):** media spreads, such as reading logs, bookshelf trackers, movie and TV trackers and sticker sheets.
- **Part 2 (items 14–28):** other bujo collections, such as art, travel, nature, food and personal memories. Items are grouped by how well they fit the app: **Strong**, **Medium** and **Weak**.

## How to use this roadmap

- Each item is **self-contained** and sized for **one fresh Claude session**. Start with: *"Implement roadmap item N from ROADMAP.md."*
- Within each part, items are ordered by **value first, then effort**; in Part 2 they are also grouped by fit. Any order works. No item needs another one done first.
- When an item is done, mark it `[x]` and add the merge commit hash.

## Shared conventions

Any item may be the first one to touch a shared piece, so each item follows these rules:

- **Extend, don't replace.** If an earlier item already made something configurable (page size, cover size, a format selector), add to it and don't rebuild it.
- **Print format selector.** Items that add a new printable (cards, spines, stubs…) put it under a **Format** control in the Print sheet tab. If the control doesn't exist yet, create it with **Cover sheet** (today's behaviour) as the default.
- **Metadata from file names.** The print sheet only sees image files, and downloads are named `Title - Creator.jpg` (with `-2`, `-3` suffixes for duplicates). An item that needs a title or creator parses it from the file name. If a parser already exists in `internal/sheet`, reuse it; otherwise add one there, with tests.
- **Defaults keep today's output.** With default settings, the existing A4 cover sheet keeps exactly today's layout: 50 mm covers, 5 × 5 grid, grey cut border.
- **Non-2:3 images.** Artworks, photos, flags and illustrations come in any shape. Any item that adds such images makes sure the print sheet places them correctly (landscape and square cells, not only shrink-to-fit into a 2:3 column). If an earlier item already did this, reuse it.
- **Attribution.** Some sources require credit (for example Wikimedia Commons CC BY images, or the Biodiversity Heritage Library). Store the credit and licence next to each download (for example a `<file>.credit.txt` sidecar) and offer a credits page or caption in the PDF. Whichever item first needs this creates it; later items reuse it.
- **Generated printables.** Some items create a page without downloading anything (map, moon calendar, quotes, trackers). They are print formats under the same **Format** control, and their input is typed in the Print sheet tab instead of a folder of images.
- **Licensing check first.** For any new source, the session starts by reading the source's official terms and API docs, and records the findings in the README. If the terms forbid the intended use, stop and report it to the user before writing code.
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

## 4. [ ] Book spines for bookshelf trackers

**Why:** bookshelf trackers (a drawn shelf filled with spines) are a very popular spread, and drawing the spines by hand is tedious.

**Scope**
- New print format **Spines**: a narrow vertical strip per item, coloured with the cover's dominant colour. The title (and optionally the creator) is rotated 90°, in black or white, whichever contrasts better with the colour.
- Dominant colour: computed from the image itself, with no new dependency.
- Spine height follows the cover height (or the size preset if item 1 is done). The width is fixed (for example 8–12 mm) or varies a little per item for a natural shelf look.

**Acceptance**
- Text is readable on light and dark colours, and long titles shrink or are truncated to fit.
- Tests for colour extraction (solid-colour fixtures) and for the contrast choice.

## 5. [ ] TV series media type

**Why:** TV trackers are as common as movie trackers. TMDB covers TV, so the existing API key can be reused.

**Scope**
- A new media type **TV series**, with input `Title, Year` (first air year optional).
- Sources: TMDB TV search (localized posters, like movies), with a no-key fallback if one is viable, such as the IMDb suggestion endpoint, which also returns series. Match on title and first air year.
- It appears in the media type selector, in the sample CSV download, in the TMDB key tip and in the CLI.

**Acceptance**
- Provider tests with recorded fixtures, as for movies. A well-known show downloads a poster with and without a TMDB key, where a fallback exists.
- README Sources section updated.

## 6. [ ] Print-then-cut support (bleed and registration marks)

**Why:** many users print onto sticker paper and cut with a Cricut or Silhouette machine, or want a small bleed so hand cutting leaves no white edges.

**Scope**
- An option **Cut mode**: **Border** (default, today's grey line), **Bleed** (each image extends 1 mm past its cut line, with crop marks), **None**.
- Optional registration marks in the page corners for cutting machines, with a note in the README on which machine settings apply.

**Acceptance**
- The default output doesn't change. With bleed on, images don't overlap their neighbours (the gap absorbs the bleed).
- Layout tests for bleed geometry.

## 7. [ ] Grayscale / ink-saver mode

**Why:** many people print on cheap home inkjets, and some journals use a black-and-white aesthetic.

**Scope**
- A checkbox **Grayscale** in the Print sheet tab. Images are converted when the PDF is built; the downloaded files stay unchanged.
- Optional: a lighter "ink saver" variant (reduced contrast).

**Acceptance**
- The PDF images are grayscale; with the box unchecked, output doesn't change. Unit test on a colour fixture.

## 8. [ ] Movie ticket stubs

**Why:** ticket stubs are a classic movie-tracker element.

**Scope**
- New print format **Ticket stubs**: a ticket shape with a perforation-style dashed line, a poster thumbnail, the title and year (from the file name), and empty "Date", "Where" and rating fields.
- It works for any media type but is labelled for movies first. Book items show the title and author.

**Acceptance**
- PDF export with several stubs per page and cut borders. Text fits as in review cards.
- Layout tests.

## 9. [ ] Full-page cover grids ("Books of 2026")

**Why:** some users want a finished page to glue or bind into the journal, with no cutting.

**Scope**
- New print format **Full page**: a header the user types (for example "Books of 2026" or "Watched in March"), then the selected covers in a tight grid that fills the page. Cover size is chosen automatically so all covers fit on one page when possible.
- No cut borders; small rounded corners or a thin frame are optional.

**Acceptance**
- 1 to about 60 covers fit on one page; beyond that, it paginates with the header repeated.
- Tests for the auto-size calculation.

## 10. [ ] TBR / watchlist checklist

**Why:** to-be-read lists and watchlists are standard bujo collections.

**Scope**
- New print format **Checklist**: one row per item with a checkbox, a small thumbnail, the title and the creator. A header the user types (for example "TBR 2026").
- Rows paginate, and the thumbnail size fits the row height.

**Acceptance**
- PDF export for books and movies, with long titles truncated. Layout tests for rows per page.

## 11. [ ] Polaroid frames

**Why:** instant-photo-style frames are a popular decoration style for media spreads.

**Scope**
- New print format **Polaroid**: each cover sits inside a white frame with a thicker bottom margin holding the title as a caption. Cut border around the frame.

**Acceptance**
- PDF export with the caption fitted to the frame. Layout tests.

## 12. [ ] Music albums media type (with square covers)

**Why:** album trackers are a natural extension, and album art is square, unlike books and posters.

**Scope**
- A new media type **Albums**, with input `Album, Artist`.
- Source: iTunes Search API (`entity=album`, artwork scaled to a large size, as for Apple Books); consider a no-key fallback such as MusicBrainz plus the Cover Art Archive.
- Print sheet: square covers get square cells (the same cover height used as the width), instead of being shrunk into the 2:3 column. Mixed folders still lay out correctly.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with fixtures. A sheet of only square covers fills the page with square cells, and the 2:3 output doesn't change.
- README Sources and Print sheet sections updated.

## 13. [ ] Video games media type

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

## 14. [ ] Museum artworks

**Why:** museum-visit logs, "artwork of the month" pages and art history collections glue in mini prints of artworks. Museums sell "museum passport" journals built around this.

**Scope**
- A new media type **Artworks**, with input `Title, Artist`. This maps directly onto the existing title and creator fields.
- Sources, in order:
  1. Art Institute of Chicago API (no key, IIIF images).
  2. The Met Collection API (no key).
  3. Cleveland Museum of Art Open Access API (no key).
- Only public-domain / CC0 images are used. Use each API's own licence field and skip anything that isn't open access.
- Match on title and artist name. Prefer the work's primary image over detail shots.
- Artworks are often landscape: follow the **Non-2:3 images** convention.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with recorded fixtures. A famous public-domain painting downloads from at least one source, and a work that isn't open access is skipped with a clear reason.
- Landscape artworks print uncropped and correctly placed. The README Sources section lists the licence rules.

## 15. [ ] Travel places (landmark photos and flags)

**Why:** travel logs and "places visited" spreads use landmark pictures, postcard-style images and flags.

**Scope**
- A new media type **Places**, with input `Place, Country` (country optional).
- Landmark or city image: Wikidata search, then the image property (P18), then the file from Wikimedia Commons, downloaded at a print-friendly size. Reuse the existing Wikidata client code where it fits.
- Optional per-item **flag** mode: when the item is a country, download its flag. Use a public-domain flag source, such as the Commons flag file linked from Wikidata.
- Commons images have their own licences: follow the **Attribution** convention.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with fixtures. A well-known landmark and a country flag both download, and their credit is stored.
- Landscape photos are placed correctly. The README explains the licence and credit handling.

## 16. [ ] Visited countries map

**Why:** a world map with visited countries coloured in is one of the most common travel spreads, and drawing one by hand is hard.

**Scope**
- A generated print format **Map**: the user picks or types the visited countries, and the app draws a world map with those countries filled. A page title is optional.
- Map data: a public-domain dataset (for example Natural Earth admin-0 countries, simplified), embedded in the binary. No network at print time.
- Options: colour, or outline only (for colouring by hand), and the page size if configurable.

**Acceptance**
- The PDF draws a clean vector map, and the selected countries are filled. Country name matching accepts common English names and ISO codes.
- Tests for name/ISO matching and for "selected countries are filled". The README notes the dataset's source and licence.

**Out of scope:** regional maps (US states, provinces). Note them as a follow-up.

## 17. [ ] Vintage nature illustrations

**Why:** bird-watching logs, plant and garden journals and nature journals use vintage natural-history plates. Collage sheets of them are very popular.

**Scope**
- A new media type **Nature**, with input `Name` (common or scientific, for example `Robin` or `Monstera deliciosa`).
- Sources: Biodiversity Heritage Library (check the API key requirement in the licensing step), with Wikimedia Commons as a fallback via the species' Wikidata entity. Prefer illustrations (plates) over photos.
- Rights are checked per item: only public-domain or clearly open images. Follow the **Attribution** convention, since BHL asks for credit with the contributing institution.
- Optional extension, in the same session if time allows: a **Plant care card** print format (illustration, name, watering, light and a note field).

**Acceptance**
- Provider tests with fixtures. A common bird and a common houseplant each return an illustration, and their credit is stored.
- The README documents the rights check and the credit format.

## Medium fit

Useful, but either demand is less proven or the item adds a new kind of printable to the app.

## 18. [ ] Moon phase calendar

**Why:** moon phase trackers are a common bujo spread (no search result confirmed it). They need no network and no licensing.

**Scope**
- A generated print format **Moon calendar**: one month or a whole year, drawn from a local astronomical calculation (phase per day, with new and full moon marked).
- Layouts: a month row (one moon per day), a month grid and a year overview.
- Options: year/month, filled or outline-only moons (for colouring), and the hemisphere (it flips how the lit side looks).

**Acceptance**
- Phase dates match a reference table, such as published new and full moon dates for a test year, within ±1 day. These checks are unit tests.
- The PDF renders all three layouts.

## 19. [ ] Anime and manga media type

**Why:** anime and manga trackers are popular among younger bujo users. This is my assumption; no search result confirmed it.

**Scope**
- A new media type **Anime/Manga**, with input `Title, Year` and an anime/manga toggle, or two media types: decide in the session.
- Source: AniList GraphQL API (no key for public queries), large cover image. Match on romaji, English and native titles.
- Covers are copyrighted, the same situation as book covers today: personal use only, as the README already implies.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with fixtures, and a well-known title downloads. Rate limits are handled through the existing throttle.

## 20. [ ] Podcasts media type

**Why:** podcast trackers sit next to reading and watching logs. The iTunes Search API the app already uses returns podcast artwork.

**Scope**
- A new media type **Podcasts**, with input `Podcast, Host` (host optional).
- Source: iTunes Search API with `media=podcast`, artwork scaled to a large size. Reuse the Apple provider's helpers.
- Artwork is square: follow the **Non-2:3 images** convention.
- Media type selector, sample CSV and CLI support.

**Acceptance**
- Provider tests with fixtures. Square artwork prints in square cells.

## 21. [ ] Photo crop to standard print sizes

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

## 22. [ ] Recipe cards

**Why:** recipe collections are a classic bujo spread. They include recipe cards, recipes-tried ratings and conversion charts.

**Weakness:** TheMealDB's free tier is for development and educational use, and its image licence is unclear. The better version doesn't download anything.

**Scope**
- A generated print format **Recipe card**: the user enters a title, ingredients, steps, servings and time (typed in, or from a CSV/Markdown file), plus an optional photo of their own. The app lays it out as a card at common sizes (A6, 4 × 6 in, or bujo-page width).
- Optional companion page: a **kitchen conversions** chart (cups/grams/ml, oven temperatures), generated.
- TheMealDB lookup only if the licensing step confirms the images may be used. Otherwise, leave it out.

**Acceptance**
- Long ingredient lists flow to a second card, or shrink within limits. Text never overflows.
- Layout tests. The README shows the input format.

## 23. [ ] Food and drink log images (Open Food Facts)

**Why:** tea, coffee, wine, beer and snack logs glue in product or label pictures.

**Weakness:** Open Food Facts images are CC BY-SA, which requires credit and share-alike, and their quality varies a lot.

**Scope**
- A new media type **Products**, with input `Product, Brand`, or a barcode.
- Source: Open Food Facts API (front image). Follow the **Attribution** convention, and record the licence per image.
- Skip images below a minimum resolution.

**Acceptance**
- Provider tests with fixtures. A common product returns a usable front image with its credit stored.

## 24. [ ] Board games media type

**Why:** board game trackers exist among hobby journalers.

**Weakness:** BoardGameGeek is the obvious source, but its API access rules may now require registration (not verified). Box art is copyrighted.

**Scope**
- **Start with the licensing/access step:** confirm BGG XML API access requirements, and present the findings and any key setup to the user before implementing.
- A new media type **Board games**, with input `Title, Year`. Source: the BGG search, then the thing image.
- Media type selector, sample CSV and CLI support. Store a token, if one is needed, like the TMDB key.

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

## 27. [ ] Quote cards

**Why:** quotes and lyrics pages are common bujo collections.

**Weakness:** there are no images to fetch, and free versions are everywhere, so the app adds little.

**Scope**
- A generated print format **Quote cards**: the user types or imports quotes (`Quote, Author`), and the app lays them out as cards in a few typographic styles using embedded open-licence fonts.

**Acceptance**
- Text auto-fits each card without overflow. Layout tests.

## 28. [ ] Habit trackers and calendars

**Why:** monthly calendars and habit trackers are core bujo pages.

**Weakness:** they are generic, free templates are everywhere, and they aren't image-based.

**Scope**
- Generated print formats **Month calendar** (any month/year; week start Monday or Sunday) and **Habit tracker** (a habit list × the days of the month grid).
- Sized to fit common bujo pages (A5, B5) and the page sizes the app supports.

**Acceptance**
- Calendar dates are correct for leap years and for both week starts (unit tests). The PDF renders both formats.
