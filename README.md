# Cover Downloader

A desktop app for Windows and Linux that downloads cover images in batches.
Paste a list of `Title, Author` lines, or import a CSV file, and it saves one cover image per item to a folder.

Book covers, movie posters and TV series posters are supported. The core is media-agnostic, so music or games can be added later as new providers.

## Features

- Batch input: one `Title, Author` (books) or `Title, Year` (movies and TV series) per line, or import a CSV file (the import replaces the list).
- Live progress: each item shows pending, saved (with file name and source), failed (with the reason) or cancelled.
- Cancel at any time. Items that have not finished are reported as cancelled.
- Fallback across sources: if one source has no match or no usable image, the next one is tried.
- Rate limits are respected: when a source answers "too many requests", the app waits as long as the source asks (or backs off) and retries. All downloads pause for that source, and the wait is shown in the app.
- Placeholder images, error pages and non-images are rejected, so you never get a blank "no cover" file.
- File names are safe on Windows and Linux (`Title - Author.jpg`). Duplicates get `-2`, `-3`, … and existing files are never overwritten.
- Store country selection (default `es`; for example `us`, `gb`, `de`). Covers of that country's editions are preferred, see [Local editions](#local-editions).
- **Open folder** shows the results in the system file manager.
- **Print sheet**: select downloaded covers and export them as an A4 PDF ready to cut out, with covers 2.5, 5 or 8 cm tall (81, 25 or 9 per page), with a grey cut border or a 1 mm bleed and crop marks for print-then-cut, as review cards with a date, stars and lines to fill in, as book spines for a bookshelf tracker, as movie ticket stubs, or as a full page of covers under a header such as "Books of 2026", sized to fit and ready to glue in. See [Print sheet](#print-sheet).
- Light and dark themes follow the system setting.
- Keyboard shortcut: **Ctrl+Enter** starts the download.
- A `covers` command-line tool that uses the same core.

## Download

Get the latest version from the [Releases page](https://github.com/iribarren/bullet-journal-covers/releases/latest).

### Windows 10/11

1. Download `cover-downloader-<version>-windows-amd64.zip` and extract it.
2. Run `cover-downloader.exe`.

The executable is not code-signed yet, so Windows SmartScreen may show **"Windows protected your PC"** the first time. Click **More info**, then **Run anyway**. WebView2 is built into Windows 10/11, so nothing else needs to be installed.

### Linux (x86-64)

1. Install the runtime libraries (Ubuntu / Debian):

   ```sh
   sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
   ```

2. Download `cover-downloader-<version>-linux-amd64.tar.gz`, then extract and run it:

   ```sh
   tar xzf cover-downloader-*-linux-amd64.tar.gz
   ./cover-downloader
   ```

The binary is built on Ubuntu 22.04, so it runs on that release and newer distributions.

## Sources

Sources are tried in this order for books:

1. **Apple Books** (iTunes Search API): large, high-quality covers. It uses the store country.
2. **Open Library**: a free, open catalog. Its own covers are often smaller, so editions with an ISBN also try the Amazon image CDN (see below).
3. **Amazon** (store search page): broad coverage, but Amazon may answer with a bot check. When that happens the item falls back to the other sources or fails with `blocked by bot check`.

Sources are tried in this order for movies:

1. **TMDB** (The Movie Database API): the best coverage and full-resolution posters, localized to the store country's language when TMDB has one (for example Spanish posters for `es`). It needs a free API key, see [TMDB API key](#tmdb-api-key). Without a key it is skipped.
2. **IMDb** (the public title-suggestion endpoint, no key): large posters for most films. It is an unofficial endpoint and may change or stop working.
3. **Wikidata → Apple TV** (no key): Wikidata links the film to its Apple TV (iTunes) movie ID, and the iTunes lookup returns the poster artwork (usually 2000 × 3000).

Sources are tried in this order for TV series (the year is the first air year, and it is optional):

1. **TMDB** TV search: the same API, key and localized posters as for movies (for example the Spanish *La casa de papel* poster for `es`). The first air year filters the search; when nothing with a poster matches, it searches again without the year. Without a key it is skipped.
2. **IMDb** (the same keyless, unofficial title-suggestion endpoint as for movies): it keeps TV series and mini-series and drops films.

There is no Wikidata → Apple TV fallback for TV series: Wikidata only links Apple TV *movie* IDs, so it cannot find show artwork.

Movie and TV posters are about 2:3, like book covers, so the print sheet lays them out the same way.

TV series use the same services, under the same terms, as movies: the TMDB API (with the attribution below) and the unofficial IMDb endpoint, with no key. Downloaded posters are for personal use.

### TMDB API key

1. Create a free account at [themoviedb.org](https://www.themoviedb.org/) and request an API key under [Settings → API](https://www.themoviedb.org/settings/api).
2. In the app, click **Settings** and paste the key ("API Key" or the longer "API Read Access Token"; both work). It is saved in your user configuration folder (`~/.config/cover-downloader/settings.json` on Linux, `%AppData%\cover-downloader\settings.json` on Windows), readable only by you.
3. For the command line, use `-tmdb-key`, or the `TMDB_API_KEY` environment variable; otherwise the key saved by the app is used.

This product uses the TMDB API but is not endorsed or certified by TMDB.

### Rate limits

All sources share one HTTP client that respects rate limits. A `429 Too Many Requests` answer (or a `503` with `Retry-After`) pauses that source for every download: the client waits for the `Retry-After` time (or 1 s, 2 s, 4 s if none is given) and retries up to 3 times. A wait longer than 60 s is not attempted; the item falls back to the next source instead. Requests to TMDB and Wikidata are also spaced out slightly.

## Local editions

The default country is `es` (Spain), so you get the covers of Spanish editions when they exist.

For each item the app makes two passes over the sources, searching each source only once:

1. **Local editions first.** Only candidates whose ISBN belongs to the country's publishers are tried, in source order. ISBNs come from Apple's artwork file names, Amazon's print-edition ASINs and Open Library editions.
2. **Any edition.** If no local edition has a usable cover, the normal order is used. This fallback is silent: the item is reported as saved, like any other.

When the country has a language, Open Library is first asked for editions in that language (for example `language:spa`). Those editions come with their ISBN, and the high-resolution cover is fetched from the Amazon image CDN by ISBN-10 (`images-na.ssl-images-amazon.com/images/P/<isbn10>.01._SCRMZZZZZZ_.jpg`), with the Open Library covers as fallbacks. This CDN is a plain image service, not the bot-checked store search.

The country table lives in [`internal/locale/locale.go`](internal/locale/locale.go):

| Country      | Open Library language | ISBN-13 prefixes             |
|--------------|-----------------------|------------------------------|
| `es`         | `spa`                 | `97884`                      |
| `mx`         | `spa`                 | `978607`, `978968`, `978970` |
| `ar`         | `spa`                 | `978950`, `978987`           |
| `fr`         | `fre`                 | `9782`                       |
| `de`         | `ger`                 | `9783`                       |
| `it`         | `ita`                 | `97888`                      |
| `pt`         | `por`                 | `978972`, `978989`           |
| `br`         | `por`                 | `97885`, `978655`            |
| `gb`, `us`   | `eng`                 | `9780`, `9781`               |

Any other country only selects the store, with no edition preference. To add a country, add a line to the table with its Open Library (MARC) language code and the ISBN-13 prefixes of its registration groups.

## CSV format

- One item per row: `title,author` for books, `title,year` for movies and TV series. The second column is optional and extra columns are ignored.
- An optional header row (for example `title,author`) is detected and skipped.
- Comma (`,`) or semicolon (`;`) delimiters are detected automatically.
- UTF-8 (with or without BOM) and Windows-1252 (the Excel default on Windows) are both accepted.
- Quote titles that contain the delimiter: `"Guns, Germs, and Steel",Jared Diamond`.

See [`sample_books.csv`](sample_books.csv) for an example. In the app, the **Sample CSV** link next to **Import CSV…** saves an example file for the selected media type (books, movies or TV series).

In the text box, a line is split on its **last** comma, so `Guns, Germs, and Steel, Jared Diamond` works without quotes. Blank lines and lines starting with `#` are ignored.

## Using the app

1. Choose the media type: **Books**, **Movies** or **TV series**. For movies and TV series, optionally add a TMDB API key in **Settings** for the best posters.
2. Type or paste items, one per line, or click **Import CSV…**.
3. Check the output folder (default: `~/Pictures/Covers`, or `%USERPROFILE%\Pictures\Covers` on Windows), and change it with **Browse…** if needed.
4. Optionally change the two-letter store country (default `es`, which prefers Spanish editions).
5. Click **Download covers** or press **Ctrl+Enter**.
6. When items finish, click **Open folder** to see the images.

## Print sheet

The **Print sheet** tab turns downloaded covers into a PDF to cut out, for example for a bullet journal, on A4, US Letter or A5 paper: a sheet of small covers, [review cards](#review-cards), [spines](#spines), [ticket stubs](#ticket-stubs), or a [full page](#full-page) of covers under a header, with nothing to cut.

1. The folder defaults to the download output folder and follows it until you pick another one with **Browse…**. The covers load when the tab opens; **Refresh** reloads them.
2. Pick a **Format**: **Cover sheet** (the default), **Review cards**, **Spines**, **Ticket stubs** or **Full page**. For review cards, pick what they are **Cards for** (Books, Movies or TV series); it follows the Download tab's media type until you change it here. For a full page, type a **Header** (for example "Books of 2026"), or leave it empty.
3. Pick a **Paper**: **A4** (the default), **US Letter** or **A5**. Each **Cover size** option shows how many covers (or spines) fit on that paper.
4. Pick a **Cover size**: **Mini**, **Standard** (the default) or **Large**. It applies to the cover sheet and spines, so it is disabled for review cards, ticket stubs and full pages.
5. For a cover sheet, pick a **Cut mode**: **Border** (the default), **Bleed + crop marks** or **None** (see [Cut mode](#cut-mode)). It never changes how many covers fit on a page.
6. Click covers (or focus one and press Space or Enter) to select them. **Select all** and **Select none** help with large folders. The toolbar shows how many covers are selected and how many pages they need with the chosen format, paper and size.
7. Click **Export PDF…**, choose where to save (the suggested name gives the format, paper, cover size and any cut mode other than Border, for example `covers_A4_Standard.pdf`, `covers_Letter_Mini.pdf`, `covers_A4_Standard_bleed.pdf`, `spines_A4_Standard.pdf` or, for review cards, ticket stubs and full pages, `cards_A4.pdf`, `stubs_A4.pdf` and `fullpage_A4.pdf`), then **Open PDF** to view it.

Covers are placed in the grid order (file name, case-insensitive, numbers in natural order). JPEG, PNG, GIF and WebP images are supported.

Grid per paper and cover size (columns × rows = covers per page):

| Paper     | Page (mm)     | Mini (25 mm) | **Standard** (50 mm) | Large (80 mm) |
|-----------|---------------|--------------|----------------------|---------------|
| **A4**    | 210 × 297     | 9 × 9 = 81   | 5 × 5 = 25           | 3 × 3 = 9     |
| US Letter | 215.9 × 279.4 | 9 × 9 = 81   | 5 × 4 = 20           | 3 × 3 = 9     |
| A5        | 148 × 210     | 6 × 6 = 36   | 3 × 3 = 9            | 2 × 2 = 4     |

On A4, for example, Standard columns are 35.6 mm wide with 15.5 mm top and bottom margins.

How the grid is derived from the cover height *h* and the page width *W* and height *H*:

| Setting           | Rule                                                                                       |
|-------------------|--------------------------------------------------------------------------------------------|
| Gap               | 4 mm between covers, both ways, at every size                                              |
| Side margins      | 8 mm                                                                                       |
| Columns           | as many as fit with every column at least ⅔·*h* wide (the 2:3 shape of a typical cover): ⌊(*W* − 2·8 + 4) / (⅔·*h* + 4)⌋ |
| Column width      | the columns then widen to fill the width between the side margins                          |
| Rows              | as many as fit with at least 8 mm top and bottom: ⌊(*H* − 2·8 + 4) / (*h* + 4)⌋             |
| Top/bottom margin | whatever height is left, split equally, so the grid is centred                             |
| Cut border        | 0.2 mm grey (#999) line exactly around each cover, in the default [cut mode](#cut-mode)    |

- **Shrink to fit:** a cover wider than its column (for example a square one) is shrunk to the column width and keeps its aspect ratio, so it ends up shorter than the chosen height. Each cover is centred in its cell. Typical book covers (about 2:3) keep the full height: at Standard size they are 50 mm tall and about 33 mm wide.
- **Pagination:** covers fill each page left to right, top to bottom. More covers than fit on a page add pages; the last page may be partly filled.
- **Print at 100 % / actual size** (not "fit to page") on every paper, so covers come out at the chosen height. Pick the paper loaded in the printer: on Letter paper choose **US Letter**, not A4, so the printer does not scale or clip the page.
- Images are downscaled to 300 dpi at their printed size (never upscaled) to keep the PDF small. If any image cannot be read, the export fails naming that file and no PDF is written. Mini covers need only about 300 px of height, and Large covers about 950 px, to print sharp.

### Cut mode

The **Cut mode** sets how a cover sheet is prepared for cutting. Review cards, spines and ticket stubs always keep their grey cut border; a full page has none.

| Cut mode               | What is printed                                                                                                   |
|------------------------|-------------------------------------------------------------------------------------------------------------------|
| **Border** (default)   | each cover with a 0.2 mm grey (#999) line on its cut line                                                         |
| Bleed + crop marks     | each cover runs 1 mm past its cut line, with crop marks at its corners and no border                              |
| None                   | the covers only, with no border and no marks                                                                      |

- **Bleed:** the 1 mm past the cut line repeats the cover's edge pixels outward, so the cover inside the cut line is exactly what Border mode prints, neither zoomed nor cropped. A slightly off cut leaves no white edge. The 4 mm gap absorbs the bleed, so neighbours never overlap.
- **Crop marks:** thin (0.1 mm) black lines that continue each cut edge outward at the four corners, from 1 mm to 2 mm past the cut line, so a mark never reaches the neighbouring cover's half of the gap. Cut along the line through two marks.
- The grid, the covers per page and the page count are the same in every cut mode. The suggested file name ends in `_bleed` or `_none` for those modes, for example `covers_A4_Standard_bleed.pdf`.

**Print then cut:** Cricut Design Space and Silhouette Studio add their own registration marks when you import a page, so the app draws none. Import the **Bleed** or **None** PDF page into the machine's app and run its print-then-cut flow there. Cricut limits the Print Then Cut area to less than a full page (see Cricut's help article "How large can I Print Then Cut?"), so a full-page sheet may need scaling or fewer covers.

### Review cards

The **Review cards** format prints one 90 × 55 mm card per selected cover, to fill in after reading or watching:

| Part       | Content                                                                                       |
|------------|-----------------------------------------------------------------------------------------------|
| Left side  | the cover                                                                                     |
| Title      | from the file name `Title - Creator.jpg`, up to 2 lines; longer text ends with "…"            |
| Creator    | the author, or the year for movies and TV series (`Title - Year.jpg`), on 1 line              |
| Date field | **Read on** or **Watched on**, from the **Cards for** choice                                  |
| Rating     | 5 empty stars                                                                                 |
| Notes      | 3 writing lines                                                                               |

Cards are centred on the page with 4 mm gaps and a grey cut border, at a fixed size (the cover size does not apply):

| Paper     | Columns × rows | Cards per page |
|-----------|----------------|----------------|
| **A4**    | 2 × 4          | 8              |
| US Letter | 2 × 4          | 8              |
| A5        | 1 × 3          | 3              |

- **File names:** the title and creator are split at the last ` - `; a name without one is a title only. A duplicate's suffix is dropped (`Dune - Frank Herbert-2.jpg` reads as "Dune" by "Frank Herbert").
- **Caveat:** a title without a creator that ends in `-2`, `-3`… is read as a duplicate suffix, so `Catch-22.jpg` prints as "Catch". Add the creator (`Catch-22 - Joseph Heller.jpg`) to keep it.

### Spines

The **Spines** format prints one book spine per selected cover, to cut out and stick on a drawn bookshelf:

| Part     | Content                                                                                          |
|----------|--------------------------------------------------------------------------------------------------|
| Strip    | filled with the cover's dominant colour, with a grey cut border                                  |
| Title    | bold, from the file name `Title - Creator.jpg`, read top to bottom                               |
| Creator  | smaller, on a second line beside the title                                                       |
| Text     | black or white, whichever contrasts more with the strip (WCAG contrast ratio)                    |

- **Height:** the **Cover size** (Mini 25, Standard 50, Large 80 mm).
- **Width:** 8 to 12 mm, in a fixed repeating pattern (10, 12, 9, 11, 8, 12, 10, 9, 11, 10 mm) for a natural shelf look, so every full row holds the same number of spines.
- **Spacing:** 2 mm between spines, 4 mm between rows, at least 8 mm margins; rows are centred on the page.
- **Long titles** shrink (down to 5 pt), then end with "…"; text never leaves its strip.
- **Dominant colour:** the colour of the largest area of similar pixels; transparent areas count as white.

Spines per page (spines per row × rows):

| Paper     | Mini (25 mm)   | **Standard** (50 mm) | Large (80 mm) |
|-----------|----------------|----------------------|---------------|
| **A4**    | 16 × 9 = 144   | 16 × 5 = 80          | 16 × 3 = 48   |
| US Letter | 16 × 9 = 144   | 16 × 4 = 64          | 16 × 3 = 48   |
| A5        | 11 × 6 = 66    | 11 × 3 = 33          | 11 × 2 = 22   |

The suggested file name is `spines_<paper>_<size>.pdf`, for example `spines_A4_Standard.pdf`.

### Ticket stubs

The **Ticket stubs** format prints one 90 × 40 mm movie ticket stub per selected cover, to keep a log of films (or books) in a journal:

| Part        | Content                                                                                    |
|-------------|--------------------------------------------------------------------------------------------|
| Outline     | grey cut border with a semicircular notch in the top and bottom edges                      |
| Perforation | a dashed line between the notches, 20 mm from the right edge                               |
| Left side   | the poster                                                                                 |
| Title       | bold, from the file name `Title - Creator.jpg`, up to 2 lines; longer text ends with "…"   |
| Creator     | the year for movies and TV series (`Title - Year.jpg`), or the author for books, on 1 line |
| Fields      | **Date:** and **Where:**, each with a line to write on                                     |
| Rating      | 5 empty stars                                                                              |
| Stub end    | **ADMIT ONE**, turned to read bottom to top                                                |

Stubs are centred on the page with 4 mm gaps, at a fixed size (the cover size and cut mode do not apply):

| Paper     | Columns × rows | Stubs per page |
|-----------|----------------|----------------|
| **A4**    | 2 × 6          | 12             |
| US Letter | 2 × 6          | 12             |
| A5        | 1 × 4          | 4              |

- The title and creator are read from the file name as for [review cards](#review-cards), with the same caveats.
- The suggested file name is `stubs_<paper>.pdf`, for example `stubs_A4.pdf`.

### Full page

The **Full page** format prints a finished page to glue or bind into a journal, with nothing to cut: the **Header** you type (for example "Books of 2026" or "Watched in March"), then the selected covers in a tight grid that fills the page. There are no cut borders.

The cover size is chosen automatically, so up to 60 covers fit on one page:

| Setting      | Rule                                                                                                     |
|--------------|----------------------------------------------------------------------------------------------------------|
| Margins      | at least 8 mm on every side                                                                              |
| Header       | bold, centred, in a 12 mm band right above the covers; 20 pt, shrunk down to 12 pt to fit the width, then cut with "…". Empty: no band, the covers use the whole height |
| Gap          | 3 mm between covers, both ways                                                                           |
| Cells        | 2:3 (the shape of a typical cover); every column count is tried and the one giving the tallest covers wins (on a tie, fewer empty cells, then more columns) |
| Cover height | at most 80 mm, so a few covers do not fill the page                                                      |
| Placement    | the grid is centred across the page, the header and grid together down it                                |

For example, on A4 with a header: 1 to 3 covers are 80 mm tall in one row, 4 are 2 × 2 at 80 mm, 12 are 4 × 3 at about 69 mm, and 60 are 8 × 8 at 31 mm.

- Covers keep their aspect ratio: a cover wider than 2:3 is shrunk to its cell's width and centred in it, as on a cover sheet.
- **More than 60 covers** go on further pages of 60, with the header repeated on every page. Every page uses the same grid, so covers keep one size, and the last page may be partly filled.
- The paper applies; the cover size and cut mode do not.
- The suggested file name is `fullpage_<paper>.pdf`, for example `fullpage_A4.pdf`.

## Using the command line

The `covers` CLI runs the same downloader without a window:

```sh
go run ./cmd/covers -o covers "Dune, Frank Herbert" "Emma, Jane Austen"
go run ./cmd/covers -country us -csv sample_books.csv -o covers
go run ./cmd/covers -type movies -o posters "Dune, 2021" "Inception, 2010"
TMDB_API_KEY=<key> go run ./cmd/covers -type movies -o posters "The Matrix, 1999"
go run ./cmd/covers -type tv -o posters "Breaking Bad, 2008" "Dark, 2017"
```

Flags must come before the items:

| Flag       | Default  | Meaning                                         |
|------------|----------|-------------------------------------------------|
| `-type`    | `books`  | Media type: `books`, `movies` or `tv`           |
| `-o`       | `covers` | Output folder                                   |
| `-country` | `es`     | Store country code; its editions are preferred  |
| `-csv`     | (none)   | CSV file, added after the positional items       |
| `-tmdb-key`| (none)   | TMDB API key; overrides `TMDB_API_KEY` and the saved key |

The exit code is `0` when every item succeeded, `1` when any item failed or was cancelled, and `2` for usage errors.

## Prerequisites

### Linux (Ubuntu / Debian)

```sh
sudo apt install golang-go nodejs npm pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Recent Ubuntu releases only ship WebKitGTK 4.1, so every `wails` command on Linux needs the `webkit2_41` build tag (`-tags webkit2_41`). `wails doctor` may still report WebKit as missing because it looks for 4.0, and that can be ignored.

### Windows 10/11

- [Go](https://go.dev/dl/) and [Node.js](https://nodejs.org/) (LTS).
- The Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
- WebView2 is built into Windows 10/11, so users need nothing else to run the `.exe`.

## Development

```sh
wails dev -tags webkit2_41        # Linux; on Windows just: wails dev
```

This runs the app with hot reload of the frontend. Wails regenerates the TypeScript bindings in `frontend/wailsjs/` when Go method signatures change (or run `wails generate module -tags webkit2_41`).

## Building

```sh
# Linux binary: build/bin/cover-downloader
wails build -tags webkit2_41

# Windows binary: build/bin/cover-downloader.exe
# Works from Windows or cross-compiled from Linux (no cgo needed).
wails build -platform windows/amd64
```

Each build is a single executable with the frontend embedded.

## Releasing

Pushing a tag that starts with `v` runs the [release workflow](.github/workflows/release.yml). It builds the Linux and Windows binaries on GitHub Actions and publishes them as a GitHub release with generated notes:

```sh
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

## Tests

```sh
go test ./internal/... ./cmd/... .
```

The root package embeds `frontend/dist`, so build the frontend once first (`cd frontend && npm install && npm run build`, or any `wails build`). On Linux, add `-tags webkit2_41` when testing the root package or running `go vet ./...`.

## Project layout

```
main.go, app.go        Wails entry point and the methods bound to the UI
frontend/              Vanilla TypeScript + Vite UI
  src/                 main.ts (download tab and tabs), print.ts (print sheet tab) and style.css
  wailsjs/             generated bindings (committed)
cmd/covers/            command-line tool
internal/
  media/               domain types: Item, Candidate, Provider, MediaType, Registry
  catalog/             built-in media types and their providers
  providers/           books: Apple Books, Open Library, Amazon; movies: TMDB, IMDb, Wikidata → iTunes; TV: TMDB, IMDb
  match/               title/creator matching and ranking
  batch/               text lines and CSV parsing
  downloader/          concurrent batch downloads, local-edition preference, image validation, file naming
  locale/              country table: language and ISBN prefixes of local editions
  isbn/                ISBN-10/13 validation, conversion and extraction
  web/                 shared HTTP client, rate-limit throttle, headers and bounded reads
  settings/            user settings file (TMDB API key)
  gallery/             print sheet: folder listing and thumbnails for the UI
  sheet/               print sheet: layouts (covers, review cards, spines, ticket stubs, full page) and PDF export
build/                 Wails build assets (icons, manifests); binaries go to build/bin/
legacy/                the original Python script
```

## Adding a new media type

1. Implement `media.Provider` (`Name()` and `Search(ctx, item, opts)`) for each source, usually in `internal/providers`. Return candidates best-first with their image URLs ordered best-first. Return an error wrapping `media.ErrNotFound` when there is no match.
2. Register a `media.MediaType` in `internal/catalog` with an ID, a label, a creator label (for example `Year` or `Artist`) and its providers in fallback order. Set `LocalEditions` only for types whose candidates carry an ISBN (books).

That's all. The UI lists media types from the registry and uses the creator label in its hints, so no frontend changes are needed.

## Legacy

`legacy/cover_downloader.py` is the original single-title Python script that this app replaces. It is kept for reference only and is not part of the build.

## License

[MIT](LICENSE)
