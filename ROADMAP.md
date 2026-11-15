# Roadmap: bullet journal printables

New printables and media types for bullet journal users, based on research into common bujo media spreads (reading logs, bookshelf trackers, movie and TV trackers, sticker sheets).

## How to use this roadmap

- Each item is **self-contained** and sized for **one fresh Claude session**. Start with: *"Implement roadmap item N from ROADMAP.md."*
- Items are ordered by **value first, then effort**. Any order works. No item needs another one done first.
- When an item is done, mark it `[x]` and add the merge commit hash.

## Shared conventions

Any item may be the first one to touch a shared piece, so each item follows these rules:

- **Extend, don't replace.** If an earlier item already made something configurable (page size, cover size, a format selector), add to it and don't rebuild it.
- **Print format selector.** Items that add a new printable (cards, spines, stubs…) put it under a **Format** control in the Print sheet tab. If the control doesn't exist yet, create it with **Cover sheet** (today's behaviour) as the default.
- **Metadata from file names.** The print sheet only sees image files, and downloads are named `Title - Creator.jpg` (with `-2`, `-3` suffixes for duplicates). An item that needs a title or creator parses it from the file name. If a parser already exists in `internal/sheet`, reuse it; otherwise add one there, with tests.
- **Defaults keep today's output.** With default settings, the existing A4 cover sheet keeps exactly today's layout: 50 mm covers, 5 × 5 grid, grey cut border.
- **Docs and tests ship with the code.** Update the README's [Print sheet](README.md#print-sheet) section (or [Sources](README.md#sources) for media types). Add table-driven Go tests next to the layout code.

Key files: `internal/sheet/layout.go` (grid and boxes), `internal/sheet/pdf.go` (PDF via fpdf), `frontend/src/print.ts` (Print sheet tab), `app.go` (Wails bindings), `internal/media`, `internal/catalog`, `internal/providers` (media types and sources).

---

## 1. [ ] Cover size presets

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

## 2. [ ] Paper sizes: A4, US Letter, A5

**Why:** US users print on Letter, and many bullet journals are A5, so people want sheets that match the journal page.

**Scope**
- A paper selector in the Print sheet tab: **A4** (default), **US Letter** (215.9 × 279.4 mm), **A5** (148 × 210 mm).
- Layout constants become per-page-size inputs. Margins, gap and cover height stay the same; columns and rows are recomputed.
- The fpdf page format follows the selection.

**Acceptance**
- A4 gives today's output. Letter and A5 produce full, centred grids with no clipped covers.
- Layout tests per paper size. The README table explains how the grid is derived.

## 3. [ ] Review cards

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
