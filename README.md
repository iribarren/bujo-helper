# BuJo Helper

Formerly "Cover Downloader".

A desktop app for Windows, macOS and Linux that downloads cover images in batches.
Paste a list of `Title, Author` lines, or import a CSV file, and it saves one cover image per item to a folder.

Book covers, movie posters, TV series posters, album covers, game box art and anime and manga covers are supported. The core is media-agnostic, so other media can be added later as new providers.

## Features

- Batch input: one `Title, Author` (books), `Title, Year` (movies, TV series, games, anime and manga), `Album, Artist` (albums), `Title, Artist` (artworks), `Place, Country` (places) or a plant or animal `Name` (nature) per line, or import a CSV file (the import replaces the list).
- Live progress: each item shows pending, saved (with file name and source), failed (with the reason) or cancelled.
- Cancel at any time. Items that have not finished are reported as cancelled.
- Fallback across sources: if one source has no match or no usable image, the next one is tried.
- Rate limits are respected: when a source answers "too many requests", the app waits as long as the source asks (or backs off) and retries. All downloads pause for that source, and the wait is shown in the app.
- Placeholder images, error pages and non-images are rejected, so you never get a blank "no cover" file.
- File names are safe on Windows and Linux (`Title - Author.jpg`). Duplicates get `-2`, `-3`, … and existing files are never overwritten.
- WebP images (some sources serve only WebP) are saved as JPEG, or as PNG when they have transparent parts, so every file opens in common viewers and print tools.
- Public-domain artworks from museum open-access collections, each saved with a credit file, see [Artworks](#artworks).
- Landmark and city photos, and country flags, from Wikimedia Commons, each saved with a credit file, see [Places](#places).
- Vintage natural-history illustrations (plates) of plants and animals from Wikimedia Commons, each saved with a credit file, see [Nature](#nature).
- Store country selection (default `es`; for example `us`, `gb`, `de`). Covers of that country's editions are preferred, see [Local editions](#local-editions).
- **Open folder** shows the results in the system file manager.
- **Print sheet**: select downloaded covers and export them as an A4 PDF ready to cut out, with covers 2.5, 5 or 8 cm tall (81, 25 or 9 per page), with a grey cut border or a 1 mm bleed and crop marks for print-then-cut, as review cards with a date, stars and lines to fill in, as book spines for a bookshelf tracker, as movie ticket stubs, as a full page of covers under a header such as "Books of 2026", sized to fit and ready to glue in, as a checklist (TBR list or watchlist) with a checkbox, thumbnail, title and creator per row, or as polaroid frames with the title as a caption. See [Print sheet](#print-sheet).
- Light and dark themes follow the system setting.
- English and Spanish interface. It follows the system language and can be changed in **Settings**, see [Language](#language).
- Keyboard shortcut: **Ctrl+Enter** starts the download.
- A `covers` command-line tool that uses the same core.

## Download

Get the latest version from the [Releases page](https://github.com/iribarren/bujo-helper/releases/latest).

### Windows 10/11

1. Download `bujo-helper-<version>-windows-amd64.zip` and extract it.
2. Run `bujo-helper.exe`.

The executable is not code-signed yet, so Windows SmartScreen may show **"Windows protected your PC"** the first time. Click **More info**, then **Run anyway**. WebView2 is built into Windows 10/11, so nothing else needs to be installed.

### macOS 11 or later (Intel and Apple Silicon)

1. Download `bujo-helper-<version>-macos-universal.zip` and open it to extract `bujo-helper.app`.
2. Move `bujo-helper.app` to **Applications**.

The app is not signed or notarized yet, so macOS blocks it the first time ("cannot be opened" or "is damaged"). To allow it, either:

- open **System Settings → Privacy & Security**, scroll to the message about `bujo-helper`, click **Open Anyway** and confirm; or
- run once in Terminal: `xattr -dr com.apple.quarantine /Applications/bujo-helper.app`.

### Linux (x86-64)

1. Install the runtime libraries (Ubuntu / Debian):

   ```sh
   sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
   ```

2. Download `bujo-helper-<version>-linux-amd64.tar.gz`, then extract and run it:

   ```sh
   tar xzf bujo-helper-*-linux-amd64.tar.gz
   ./bujo-helper
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

Sources are tried in this order for albums (the artist is optional, but it helps):

1. **MusicBrainz + Cover Art Archive** (no key): the MusicBrainz API finds the release group (the album across all its editions) by title and artist, preferring albums over singles and other releases with the same title. The [Cover Art Archive](https://coverartarchive.org/) serves its front cover at 1200 px, falling back to 500 px and then the original image.
2. **Apple Music** (iTunes Search API, album search): large, high-quality artwork. It uses the store country.

Album covers are square, unlike the 2:3 book covers and posters, so a cover sheet gives them square cells (see [Print sheet](#print-sheet)).

MusicBrainz data is CC0, but the Cover Art Archive images carry no licence of their own: its documentation says to use them at your own risk. Apple's terms allow iTunes artwork only to promote the store's content, the same terms as the Apple Books and Apple TV sources above. Downloaded covers are for personal use.

MusicBrainz allows at most one request per second and asks every client for a User-Agent with contact information. The app sends `bujo-helper/1.0 ( https://github.com/iribarren/bujo-helper )` and spaces its MusicBrainz requests one second apart, so a long album list takes at least a second per album.

Sources are tried in this order for games (the year is the first release year, and it is optional):

1. **IGDB** (the Internet Game Database API, owned by Twitch): box art for games on every platform, at 1080p, then 720p, then the 528 × 748 cover. It needs your own free Twitch app credentials, see [IGDB credentials](#igdb-credentials). Without them it is skipped.
2. **Steam** (the store search, no key): the portrait library art (600 × 900, about 300 dpi at the Standard 50 mm size; 300 × 450 when the larger one is missing). It only knows PC games sold on Steam. Its search has no release dates, so when several results share the best title and a year is given, the app looks up the release dates of up to three of them to pick that year.

Game covers are about 3:4 (IGDB) or 2:3 (Steam). The print sheet fits them into the same 2:3 cells as book covers.

The IGDB API is free for non-commercial use under the [Twitch Developer Services Agreement](https://www.twitch.tv/p/legal/developer-agreement/); commercial use needs a partnership with IGDB. The Steam store search is undocumented, like the IMDb endpoint, and may change or stop working; Valve's terms grant no rights to its images. This app is not endorsed by Twitch, IGDB or Valve. Downloaded covers are for personal use.

### Anime and Manga

Anime and manga covers come from **AniList** (its GraphQL API, no key). The year is the start year (the first air or publication year), and it is optional.

- One search per item asks for up to 10 non-adult results of the selected type (anime or manga), in AniList's relevance order. Results without a cover are skipped.
- Titles match on every form AniList knows: romaji (`Shingeki no Kyojin`), English (`Attack on Titan`), native (`進撃の巨人`) and synonyms (`AoT`). Without a year, a result with an exact match on any of them comes first, then the closest title matches, in AniList's order on ties. A year picks among the matches, as for games: `Berserk, 2016` gives the 2016 anime instead of the 1997 one, and `Berserk, 2012` gives the 2012 Golden Age Arc film, whose main title matches.
- The file is named after the title you typed, like other media types.
- The cover is the extra-large image (about 460 × 657), falling back to the large one. Covers are about 2:3, like book covers.

The [AniList API terms](https://anilist.gitbook.io/anilist-apiv2-docs/) make it free for non-commercial use and forbid using it as a backup or data store, or hoarding (mass collection of) its data. The app only searches the titles you ask for, one at a time. AniList allows 90 requests per minute, currently degraded to 30, and blocks a client that goes over for a minute; the app spaces its AniList requests 2 s apart (30 per minute). This app is not endorsed by AniList. Cover images are copyrighted by their publishers; downloaded covers are for personal use only.

### Artworks

Artworks download only public-domain images, from museum open-access collections. Paintings come in every shape, so a cover sheet gives landscape ones wide cells (see [Print sheet](#print-sheet)). Sources are tried in this order (the artist is optional, but it helps):

1. **The Metropolitan Museum of Art** (Collection API, no key): searches titles first. When that finds nothing, or a common title matches more than 10 works and an artist is given, it searches the title and artist together instead (ranked by relevance). It looks at up to 10 works and takes the original JPEG, falling back to the web-size one.
2. **The Cleveland Museum of Art** (Open Access API, no key): searches CC0 works with images by title and artist, and again by title alone when the artist filter finds nothing. It takes the print image (about 3400 px), falling back to the web image (about 900 px).

Only a work whose title matches (exactly, or as the start of the museum's title) and, when an artist is given, whose artist contains its surname is kept. A museum search matches any word of the title, so this stops a different work from being saved; the next source is tried instead.

**Licence rules.** A work is downloaded only when the museum marks it as open access:

| Museum    | Kept when                            | Licence of the image  |
|-----------|--------------------------------------|-----------------------|
| The Met   | `isPublicDomain` and a primary image | CC0 (Met Open Access) |
| Cleveland | `share_license_status` is `CC0`      | CC0                   |

A matching work that is not open access is skipped with a reason such as `met: not found: 1 match, none with a public-domain (CC0) image`. CC0 allows any use without attribution, but the app still records where each image comes from.

**Credit file.** Each artwork image is saved with a credit file next to it: the image's file name plus `.credit.txt`, including any duplicate suffix (`Wheat Field with Cypresses - Vincent van Gogh-2.jpg.credit.txt`). For example:

```text
Title: Wheat Field with Cypresses (1889)
Artist: Vincent van Gogh
Museum: The Metropolitan Museum of Art
License: CC0 1.0 Universal (public domain), https://creativecommons.org/publicdomain/zero/1.0/
Object page: https://www.metmuseum.org/art/collection/search/436535
```

Place and nature images get one too (see [Places](#places) and [Nature](#nature)); other media types get no credit file. The print sheet can list them on a [credits page](#credits-page). The print sheet gallery lists images only, never credit files. The app never deletes or renames downloaded images; if you do, do the same to the credit file.

**Why not the Art Institute of Chicago.** Its API and public-domain images are CC0 too, but its image server (`www.artic.edu/iiif/2/…`) answers every request that does not come from a browser with a Cloudflare challenge (HTTP 403, checked in November 2026), so the app cannot download its images.

The Met asks for at most 80 requests per second; a search makes up to 12 requests, which the app spaces 25 ms apart. Cleveland states no limit.

### Places

Places download a photo of a landmark, a monument or a city from [Wikimedia Commons](https://commons.wikimedia.org/), found through [Wikidata](https://www.wikidata.org/) (no key). The country is optional, but it helps when a name exists in several countries (`Córdoba, Spain` or `Córdoba, Argentina`). Photos come in every shape and flags are usually 3:2, so a cover sheet gives landscape ones wide cells (see [Print sheet](#print-sheet)).

1. **Wikidata** full-text search for the place among items that have an image (property P18), up to 10 results.
2. When a country is given, the results in that country (property P17, or a description that names it; `USA` and `UK` also work) come first; the others are kept after them. Within each group, the closest name comes first.
3. **Commons** returns the item's image, up to three results: the 1920 px thumbnail, falling back to 1280 px. Both are standard Wikimedia thumbnail widths; an image narrower than 1920 px comes as the original.

**Flag mode.** An item written `Flag of Japan` or `Japan flag` (any case) downloads the country's flag (property P41) instead of a photo, and its country column is ignored. Flags are SVG files on Commons, so they come as 1920 px PNG thumbnails.

**Licences.** Wikidata's data is CC0, but that does not cover the images: every Commons file keeps its own licence, often CC BY or CC BY-SA, sometimes public domain. Commons allows reuse in print, as long as you credit the author (not the uploader) and name the licence. The app stores the licence as Commons gives it; Wikimedia gives no warranty that it is right, so check the file page before you publish.

**Credit file.** Each place image is saved with a credit file next to it, the same way as artworks: the image's file name plus `.credit.txt`. It holds the image title, the author (as the author asks to be credited, when the file says so), the source, the licence (or "public domain"), the Commons file page and the Wikidata item. For example, for `Córdoba, Spain`:

```text
Title: Centro Histórico, 16.9 -- 2023 -- Córdoba, España
Author: Jose María Ligero Loarte
Source: Own work
License: CC BY-SA 4.0, https://creativecommons.org/licenses/by-sa/4.0
File page: https://commons.wikimedia.org/wiki/File:Centro_Hist%C3%B3rico,_16.9_--_2023_--_C%C3%B3rdoba,_Espa%C3%B1a.jpg
Wikidata item: https://www.wikidata.org/wiki/Q5818
```

When you print or share a page with these images, credit each one with its author and licence. The print sheet's [Credits page](#credits-page) does this for you.

Wikimedia asks every client for a User-Agent with contact information, also when downloading images. The app sends `bujo-helper/1.0 ( https://github.com/iribarren/bujo-helper )` to Wikidata, the Commons API and the Commons image servers, spaces Wikidata and Commons API requests 200 ms apart, and image downloads from Commons 500 ms apart.

### Nature

Nature downloads a vintage natural-history illustration of a plant or an animal, a plate from an old book rather than a photo, from [Wikimedia Commons](https://commons.wikimedia.org/) (no key), then from the [Biodiversity Heritage Library](https://www.biodiversitylibrary.org/) (BHL) when you have set a free BHL API key (see [BHL API key](#bhl-api-key)). Write one name per line, common or scientific: `Robin`, `Barn owl`, `Monstera deliciosa`. There is no second column. Plates come in every shape, so a cover sheet gives landscape ones wide cells (see [Print sheet](#print-sheet)).

1. **Wikidata** full-text search for the name among taxa (items that are an instance of taxon), up to 10 results. The result whose English name or alias (aliases hold other common names and the scientific name) is closest to the name you wrote is used; ties keep the search order, so `Robin` is the European robin (*Erithacus rubecula*). A name that matches no taxon closely enough fails with "no Wikidata taxon named …". Each name is looked up once while the app (or the `covers` command) runs.
2. **Commons** search for files, in this order, until one search gives an openly licensed image:
   1. files that depict the taxon and are marked public domain (structured data: depicts P180, copyright status P6216);
   2. the taxon's Commons category followed by ` - botanical illustrations` (plants);
   3. files that mention the scientific name and the word "plate".
3. Of each search's first files (those with "Plate" in the name first), Commons returns up to five with their licence. Files with "Plate" in the name or scanned by the [Biodiversity Heritage Library](https://www.biodiversitylibrary.org/) (BHL) come first, the rest keep the search order, up to three results: the 1920 px thumbnail, falling back to 1280 px; an image narrower than 1920 px comes as the original.

**Rights check.** Only public domain, CC0, CC BY and CC BY-SA files are used, since they allow printing with a credit. Files under a non-commercial (NC) or no-derivatives (ND) licence, or with no licence information, are skipped; when nothing is left the item fails and names the skipped files with the reason, for example `Monstera deliciosa.png: no-derivatives licence (CC BY-ND 4.0)`. As for places, the app stores the licence as Commons gives it and Wikimedia gives no warranty that it is right, so check the file page before you publish.

**Credit file.** Each nature image is saved with a credit file next to it, the same way as places: the image title, the author, the date, the source, the licence, the Commons file page and the Wikidata taxon, plus the line BHL asks for when the scan comes from BHL. For example, for `Robin`:

```text
Title: Coloured figures of the birds of the British Islands - issued by Lord Lilford (10036178774)
Author: Keulemans, J. G.; Lilford, Thomas Littleton Powys; Newton, Alfred; Salvin, Osbert; Thorburn, Archibald
Date: between 1885 and 1897
Source: https://www.flickr.com/photos/biodivlibrary/10036178774
License: Public domain
File page: https://commons.wikimedia.org/wiki/File:Coloured_figures_of_the_birds_of_the_British_Islands_-_issued_by_Lord_Lilford_(10036178774).jpg
Wikidata taxon: https://www.wikidata.org/wiki/Q25334
Image from the Biodiversity Heritage Library
```

Public domain images need no credit, but BHL asks for one and it is good practice; the print sheet's [Credits page](#credits-page) lists them. Requests go to Wikidata and Commons with the same contact User-Agent and spacing as places.

**Biodiversity Heritage Library (with a key).** When Commons gives no open image and a BHL API key is set, BHL is searched next; without a key it is skipped, and a name Commons cannot illustrate is still reported as not found (the skipped BHL search is not listed as a reason). It uses the same Wikidata taxon (looked up once for both sources) and needs its scientific name (Wikidata P225); a taxon without one is not searched.

1. **GetNameMetadata** lists the scanned pages that name the taxon (BHL finds names in the OCR text, so most are text pages). Only pages typed *Illustration* or numbered as a *Plate* are kept, grouped by volume, oldest publication first. A name with thousands of pages can make a large answer; one over 16 MB fails that item's BHL search with "response body too large".
2. **GetItemMetadata** gives each volume's rights, for up to three volumes. The first volume that passes the rights check gives up to three of its plates: the full page scan (`https://www.biodiversitylibrary.org/pageimage/<page ID>`). BHL serves scans only as WebP; the app saves them as JPEG.

**BHL rights check.** A volume is used when its copyright status is public domain, `NOT_IN_COPYRIGHT`, not specified (or blank), or "No known copyright restrictions", and it has no non-commercial (NC) or no-derivatives (ND) licence. Volumes "In copyright" (usually digitized with the rights holder's permission under CC BY-NC-SA) and any other status are skipped; when nothing is left the item fails and names the skipped volumes with the reason, for example `Bird notes (item 401): in copyright (In copyright. Digitized with the permission of the rights holder.)`.

**BHL credit file.** It starts with the line BHL asks for, then the title and volume, the publication date, the page, the copyright status, the licence and rights links when BHL gives them, the volume page and the Wikidata taxon. For example (illustrative item and page IDs):

```text
Image from the Biodiversity Heritage Library. Contributed by Natural History Museum Library, London. | www.biodiversitylibrary.org
Title: Coloured figures of the birds of the British Islands, v.1
Date: 1885-1897
Page: https://www.biodiversitylibrary.org/page/20013 (Plate 4)
Copyright status: NOT_IN_COPYRIGHT
Item: https://www.biodiversitylibrary.org/item/201
Wikidata taxon: https://www.wikidata.org/wiki/Q25334
```

BHL metadata is CC0. API requests send the contact User-Agent; the key travels in the request URL and is never shown in error messages.

### TMDB API key

1. Create a free account at [themoviedb.org](https://www.themoviedb.org/) and request an API key under [Settings → API](https://www.themoviedb.org/settings/api).
2. In the app, click **Settings** and paste the key ("API Key" or the longer "API Read Access Token"; both work). It is saved in your user configuration folder (`~/.config/bujo-helper/settings.json` on Linux, `%AppData%\bujo-helper\settings.json` on Windows, `~/Library/Application Support/bujo-helper/settings.json` on macOS), readable only by you. Settings saved by older versions in the `cover-downloader` folder are copied over automatically on first launch.
3. For the command line, use `-tmdb-key`, or the `TMDB_API_KEY` environment variable; otherwise the key saved by the app is used.

This product uses the TMDB API but is not endorsed or certified by TMDB.

### IGDB credentials

1. Sign in at [dev.twitch.tv](https://dev.twitch.tv/) with a Twitch account. Twitch requires two-factor authentication to register apps.
2. In the [Developer Console](https://dev.twitch.tv/console/apps), click **Register Your Application**: any name, OAuth Redirect URL `http://localhost`, a category such as *Application Integration*, and Client Type **Confidential**.
3. Open the app with **Manage**, copy its **Client ID**, and click **New Secret** to get a **Client Secret**.
4. In the app, click **Settings** and paste both into **IGDB Client ID** and **IGDB Client Secret**. They are saved next to the TMDB key, readable only by you. The app exchanges them for an access token (valid for about two months), which it keeps in memory only.
5. For the command line, use `-igdb-id` and `-igdb-secret`, or the `IGDB_CLIENT_ID` and `IGDB_CLIENT_SECRET` environment variables; otherwise the credentials saved by the app are used.

### BHL API key

The Biodiversity Heritage Library key is optional: nature images come from Wikimedia Commons without it, and BHL adds plates when Commons has none.

1. Request a free key at [biodiversitylibrary.org/getapikey.aspx](https://www.biodiversitylibrary.org/getapikey.aspx) (a name and an email address); BHL emails it to you.
2. In the app, click **Settings** and paste it into **BHL API key**. It is saved next to the TMDB key, readable only by you, and sent only to the BHL API (in the request URL, as BHL requires).
3. For the command line, use `-bhl-key`, or the `BHL_API_KEY` environment variable; otherwise the key saved by the app is used.

While **Nature** is selected and no key is saved, a tip above the list suggests adding one. A key BHL rejects fails the item's BHL search with "BHL rejected the API key (HTTP 401)".

### Rate limits

All sources share one HTTP client that respects rate limits. A `429 Too Many Requests` answer (or a `503` with `Retry-After`) pauses that source for every download: the client waits for the `Retry-After` time (or 1 s, 2 s, 4 s if none is given) and retries up to 3 times. A wait longer than 60 s is not attempted; the item falls back to the next source instead. Requests to TMDB, Wikidata and the Commons API are also spaced out slightly, requests to IGDB 250 ms apart (IGDB allows four per second), requests to AniList 2 s apart (AniList allows 30 per minute while degraded), requests to MusicBrainz one second apart, requests to the Met 25 ms apart, image downloads from Wikimedia Commons 500 ms apart, and requests to the BHL API and its page scans 500 ms apart (BHL has IP rate limits but publishes no numbers). A nature item makes at most three Wikidata requests (once per name) and two Commons API requests per search, at most three searches; with a BHL key, BHL adds one name request and up to three volume requests. IGDB and AniList searches are POST requests; they are retried like the others, since a rate-limited request was not processed.

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

- One item per row: `title,author` for books, `title,year` for movies, TV series, games, anime and manga, `album,artist` for albums, `title,artist` for artworks, `place,country` for places, and a single `name` column for nature. The second column is optional and extra columns are ignored.
- An optional header row (for example `title,author`) is detected and skipped.
- Comma (`,`) or semicolon (`;`) delimiters are detected automatically.
- UTF-8 (with or without BOM) and Windows-1252 (the Excel default on Windows) are both accepted.
- Quote titles that contain the delimiter: `"Guns, Germs, and Steel",Jared Diamond`.

See [`sample_books.csv`](sample_books.csv) for an example. In the app, the **Sample CSV** link next to **Import CSV…** saves an example file for the selected media type (books, movies, TV series, albums, games, anime, manga, artworks, places or nature).

In the text box, a line is split on its **last** comma, so `Guns, Germs, and Steel, Jared Diamond` works without quotes. Blank lines and lines starting with `#` are ignored.

## Using the app

1. Choose the media type: **Books**, **Movies**, **TV series**, **Albums** or **Games**. For movies and TV series, optionally add a TMDB API key in **Settings** for the best posters; for games, add IGDB credentials for box art beyond PC games; for nature, a BHL API key adds plates when Commons has none.
2. Type or paste items, one per line, or click **Import CSV…**.
3. Check the output folder (default: `~/Pictures/Covers`, or `%USERPROFILE%\Pictures\Covers` on Windows), and change it with **Browse…** if needed.
4. Optionally change the two-letter store country (default `es`, which prefers Spanish editions).
5. Click **Download covers** or press **Ctrl+Enter**.
6. When items finish, click **Open folder** to see the images.

## Print sheet

The **Print sheet** tab turns downloaded covers into a PDF to cut out, for example for a bullet journal, on A4, US Letter or A5 paper: a sheet of small covers, [review cards](#review-cards), [spines](#spines), [ticket stubs](#ticket-stubs), a [full page](#full-page) of covers under a header, with nothing to cut, a [checklist](#checklist) to tick off as you read or watch, or [polaroids](#polaroid) with the title as a caption.

1. The folder defaults to the download output folder and follows it until you pick another one with **Browse…**. The covers load when the tab opens; **Refresh** reloads them.
2. Pick a **Format**: **Cover sheet** (the default), **Review cards**, **Spines**, **Ticket stubs**, **Full page**, **Checklist** or **Polaroid**. For review cards, pick what they are **Cards for** (Books, Movies, TV series or Albums); it follows the Download tab's media type until you change it here. For a full page or a checklist, type a **Header** (for example "Books of 2026" or "TBR 2026"), or leave it empty.
3. Pick a **Paper**: **A4** (the default), **US Letter** or **A5**. Each **Cover size** option shows how many covers (or spines, or polaroids) fit on that paper. On a cover sheet it follows the selection: when every selected image is square (albums), how many square covers fit; when every one gets a wide cell at that size (landscape artworks), how many images of the selection's middle shape fit, which is exact when they all share one shape; otherwise, and for mixed shapes, how many 2:3 covers fit.
4. Pick a **Cover size**: **Mini**, **Standard** (the default) or **Large**. It applies to the cover sheet, spines and polaroids, so it is disabled for review cards, ticket stubs, full pages and checklists.
5. For a cover sheet, pick a **Cut mode**: **Border** (the default), **Bleed + crop marks** or **None** (see [Cut mode](#cut-mode)). It never changes how many covers fit on a page.
6. Leave **Credits page** checked (the default) to end the PDF with the author and licence of every selected image that has a credit file, in every format (see [Credits page](#credits-page)). Uncheck it to print the images only.
7. Click covers (or focus one and press Space or Enter) to select them. **Select all** and **Select none** help with large folders. The toolbar shows how many covers are selected and how many pages they need with the chosen format, paper and size (and, for a checklist, header), including any credits pages. The page count is exact: it reads the size of each selected image, since square and landscape covers take more room on a cover sheet (an image that cannot be read counts as a 2:3 cover).
8. Click **Export PDF…**, choose where to save (the suggested name gives the format, paper, cover size and any cut mode other than Border, for example `covers_A4_Standard.pdf`, `covers_Letter_Mini.pdf`, `covers_A4_Standard_bleed.pdf`, `spines_A4_Standard.pdf`, `polaroid_A4_Standard.pdf` or, for review cards, ticket stubs, full pages and checklists, `cards_A4.pdf`, `stubs_A4.pdf`, `fullpage_A4.pdf` and `checklist_A4.pdf`), then **Open PDF** to view it.

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

- **Square cells:** a square cover (width / height within about 10 % of 1, such as album art) gets a square cell, *h* × *h*, instead of a column. Cells flow left to right, 4 mm apart, and a row wraps when the next cell does not fit, so a folder of books and albums mixes both kinds of cell. The rows per page stay the same. Each row is centred, and a short row is centred as if it were filled with more cells like its last one, so the last row lines up with the full rows above it. A sheet whose covers all fit the 2:3 column keeps the grid in the table above.
- **Wide cells:** a cover wider than the column at full height that is not square gets a wide cell, as wide as the cover at height *h*: a 3:2 painting at Standard size gets a 75 × 50 mm cell, a 4:5 portrait 40 × 50 mm. Wide cells flow, wrap and centre like square cells. No cell is wider than the space between the side margins: a panorama wider than that shrinks to fit, so it is shorter than *h* and centred in its row (a 5:1 panorama on A4 Standard is 194 × 38.8 mm). Whether a 4:5 portrait needs a wide cell depends on the column: on A5 Standard and Large it fits the column.
- **3:2 covers per page:** A4 4 × 9 = 36 (Mini), 2 × 5 = 10 (Standard), 1 × 3 = 3 (Large); US Letter 4 × 9 = 36, 2 × 4 = 8, 1 × 3 = 3; A5 3 × 6 = 18, 1 × 3 = 3, 1 × 2 = 2.
- **Square covers per page:** A4 6 × 9 = 54 (Mini), 3 × 5 = 15 (Standard), 2 × 3 = 6 (Large); US Letter 7 × 9 = 63, 3 × 4 = 12, 2 × 3 = 6; A5 4 × 6 = 24, 2 × 3 = 6, 1 × 2 = 2.
- **Shrink to fit:** a cover wider than its cell (a near-square cover in its square cell, or a panorama wider than the page) is shrunk to the cell width and keeps its aspect ratio, so it ends up shorter than the chosen height. Each cover is centred in its cell. Typical book covers (about 2:3) keep the full height: at Standard size they are 50 mm tall and about 33 mm wide. The other formats have no square or wide cells: square and landscape images shrink to fit there.
- **Pagination:** covers fill each page left to right, top to bottom. More covers than fit on a page add pages; the last page may be partly filled.
- **Print at 100 % / actual size** (not "fit to page") on every paper, so covers come out at the chosen height. Pick the paper loaded in the printer: on Letter paper choose **US Letter**, not A4, so the printer does not scale or clip the page.
- Images are downscaled to 300 dpi at their printed size (never upscaled) to keep the PDF small. If any image cannot be read, the export fails naming that file and no PDF is written. Mini covers need only about 300 px of height, and Large covers about 950 px, to print sharp.

### Cut mode

The **Cut mode** sets how a cover sheet is prepared for cutting. Review cards, spines, ticket stubs and polaroids always keep their grey cut border; a full page and a checklist have none.

| Cut mode               | What is printed                                                                                                   |
|------------------------|-------------------------------------------------------------------------------------------------------------------|
| **Border** (default)   | each cover with a 0.2 mm grey (#999) line on its cut line                                                         |
| Bleed + crop marks     | each cover runs 1 mm past its cut line, with crop marks at its corners and no border                              |
| None                   | the covers only, with no border and no marks                                                                      |

- **Bleed:** the 1 mm past the cut line repeats the cover's edge pixels outward, so the cover inside the cut line is exactly what Border mode prints, neither zoomed nor cropped. A slightly off cut leaves no white edge. The 4 mm gap absorbs the bleed, so neighbours never overlap.
- **Crop marks:** thin (0.1 mm) black lines that continue each cut edge outward at the four corners, from 1 mm to 2 mm past the cut line, so a mark never reaches the neighbouring cover's half of the gap. Cut along the line through two marks.
- The grid, the covers per page and the page count are the same in every cut mode. The suggested file name ends in `_bleed` or `_none` for those modes, for example `covers_A4_Standard_bleed.pdf`.

**Print then cut:** Cricut Design Space and Silhouette Studio add their own registration marks when you import a page, so the app draws none. Import the **Bleed** or **None** PDF page into the machine's app and run its print-then-cut flow there. Cricut limits the Print Then Cut area to less than a full page (see Cricut's help article "How large can I Print Then Cut?"), so a full-page sheet may need scaling or fewer covers.

### Credits page

Wikimedia Commons images (see [Places](#places) and [Nature](#nature)) are often CC BY or CC BY-SA, which require the author and licence wherever the image is reproduced, printed sheets included; artworks are public domain, but credit is still good practice. With **Credits page** checked (the default), the PDF ends with one or more pages titled "Credits" ("Créditos" in Spanish), on the same paper, listing every selected image that has a credit file (`<image>.credit.txt`, see [Artworks](#artworks), [Places](#places) and [Nature](#nature)): a number, the image's title and creator from its file name, and the credit text, wrapped to the page width, in the order the images appear in the sheet.

- Images without a credit file (books, movies, TV series, albums, games, anime, manga) are not listed. When no selected image has one, or the box is unchecked, no page is added and the PDF is exactly what it would be without the option.
- A credit file that exists but cannot be read stops the export, naming the file, like an unreadable image.
- Credits print in the embedded Go font, which covers Latin, Greek and Cyrillic text; characters it lacks (for example Japanese) print as empty boxes, though they can still be copied from the PDF, and emoji print as "?".

### Review cards

The **Review cards** format prints one 90 × 55 mm card per selected cover, to fill in after reading or watching:

| Part       | Content                                                                                       |
|------------|-----------------------------------------------------------------------------------------------|
| Left side  | the cover                                                                                     |
| Title      | from the file name `Title - Creator.jpg`, up to 2 lines; longer text ends with "…"            |
| Creator    | the author, the year for movies and TV series (`Title - Year.jpg`), or the artist for albums, on 1 line |
| Date field | **Read on**, **Watched on** or **Listened on**, from the **Cards for** choice                 |
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
| Creator     | the year for movies and TV series (`Title - Year.jpg`), the author for books, or the artist for albums, on 1 line |
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

### Checklist

The **Checklist** format prints a to-be-read list or watchlist to glue into a journal: the **Header** you type (for example "TBR 2026" or "Watchlist"), then one row per selected cover to tick off. There are no cut borders.

| Part      | Content                                                                                       |
|-----------|-----------------------------------------------------------------------------------------------|
| Header    | as on a [full page](#full-page): bold, centred, in a 12 mm band, shrunk then cut with "…" to fit; repeated on every page. Empty: no band |
| Checkbox  | a 5 mm square on the left                                                                     |
| Thumbnail | the cover in a 2:3 box 13 mm tall, keeping its aspect ratio (square and wide images are shrunk to fit and centred) |
| Title     | bold, from the file name `Title - Creator.jpg`, on 1 line; longer text ends with "…"          |
| Creator   | the year for movies and TV series (`Title - Year.jpg`), the author for books, or the artist for albums, on 1 line under the title; without one, the title is centred alone |
| Rule      | a thin light-grey line under each row                                                         |

Rows are 16 mm tall and span the page within 8 mm margins, from the top margin down, or right below the header:

| Paper     | Rows per page | With a header |
|-----------|---------------|---------------|
| **A4**    | 17            | 16            |
| US Letter | 16            | 15            |
| A5        | 12            | 11            |

- More rows go on further pages, with the header repeated on every page; the last page may be partly filled.
- The title and creator are read from the file name as for [review cards](#review-cards), with the same caveats.
- The paper applies; the cover size and cut mode do not.
- The suggested file name is `checklist_<paper>.pdf`, for example `checklist_A4.pdf`.

### Polaroid

The **Polaroid** format prints each selected cover in a white instant-photo frame, with its title as a caption in the wider bottom margin, to cut out and stick in a journal:

| Part    | Content                                                                                                  |
|---------|----------------------------------------------------------------------------------------------------------|
| Frame   | white, with a grey cut border around it                                                                  |
| Picture | the cover in a 2:3 window as tall as the **Cover size**, keeping its aspect ratio (square and wide images are shrunk to fit and centred) |
| Caption | the title from the file name `Title - Creator.jpg` (no creator), centred in the bottom margin, up to 2 lines; longer text ends with "…". No title: no caption |

The frame follows the cover size *h*: a 0.08·*h* border at the top and sides, a 0.28·*h* bottom margin, and a caption of 0.18 pt per mm of *h*, kept between 6 and 12 pt:

| Size                 | Picture (mm) | Frame (mm)   | Bottom margin | Caption |
|----------------------|--------------|--------------|---------------|---------|
| Mini (25 mm)         | 16.7 × 25    | 20.7 × 34    | 7 mm          | 6 pt    |
| **Standard** (50 mm) | 33.3 × 50    | 41.3 × 68    | 14 mm         | 9 pt    |
| Large (80 mm)        | 53.3 × 80    | 66.1 × 108.8 | 22.4 mm       | 12 pt   |

Frames are centred on the page with 4 mm gaps and at least 8 mm margins. Polaroids per page (columns × rows):

| Paper     | Mini       | **Standard** | Large     |
|-----------|------------|--------------|-----------|
| **A4**    | 8 × 7 = 56 | 4 × 3 = 12   | 2 × 2 = 4 |
| US Letter | 8 × 7 = 56 | 4 × 3 = 12   | 2 × 2 = 4 |
| A5        | 5 × 5 = 25 | 3 × 2 = 6    | 1 × 1 = 1 |

- The title is read from the file name as for [review cards](#review-cards), with the same caveats.
- The paper and cover size apply; the cut mode and header do not (the border always stays).
- The suggested file name is `polaroid_<paper>_<size>.pdf`, for example `polaroid_A4_Standard.pdf`.

## Using the command line

The `covers` CLI runs the same downloader without a window:

```sh
go run ./cmd/covers -o covers "Dune, Frank Herbert" "Emma, Jane Austen"
go run ./cmd/covers -country us -csv sample_books.csv -o covers
go run ./cmd/covers -type movies -o posters "Dune, 2021" "Inception, 2010"
TMDB_API_KEY=<key> go run ./cmd/covers -type movies -o posters "The Matrix, 1999"
go run ./cmd/covers -type tv -o posters "Breaking Bad, 2008" "Dark, 2017"
go run ./cmd/covers -type albums -o albums "Abbey Road, The Beatles" "Rumours, Fleetwood Mac"
go run ./cmd/covers -type games -o games "Hollow Knight, 2017" "Celeste, 2018"
IGDB_CLIENT_ID=<id> IGDB_CLIENT_SECRET=<secret> go run ./cmd/covers -type games -o games "The Legend of Zelda: Breath of the Wild, 2017"
go run ./cmd/covers -type anime -o anime "Frieren, 2023" "Shingeki no Kyojin, 2013"
go run ./cmd/covers -type manga -o manga "Berserk, 1989" "Vagabond, 1998"
go run ./cmd/covers -type artworks -o art "Wheat Field with Cypresses, Vincent van Gogh" "Twilight in the Wilderness, Frederic Edwin Church"
go run ./cmd/covers -type places -o places "Eiffel Tower, France" "Machu Picchu, Peru" "Flag of Japan"
go run ./cmd/covers -type nature -o nature "Robin" "Monstera deliciosa" "Barn owl"
BHL_API_KEY=<key> go run ./cmd/covers -type nature -o nature "Barn owl"
```

Flags must come before the items:

| Flag           | Default  | Meaning                                                    |
|----------------|----------|------------------------------------------------------------|
| `-type`        | `books`  | Media type: `books`, `movies`, `tv`, `albums`, `games`, `anime`, `manga`, `artworks`, `places` or `nature` |
| `-o`           | `covers` | Output folder                                              |
| `-country`     | `es`     | Store country code; its editions are preferred             |
| `-csv`         | (none)   | CSV file, added after the positional items                 |
| `-tmdb-key`    | (none)   | TMDB API key; overrides `TMDB_API_KEY` and the saved key   |
| `-igdb-id`     | (none)   | IGDB (Twitch) Client ID; overrides `IGDB_CLIENT_ID` and the saved one |
| `-igdb-secret` | (none)   | IGDB (Twitch) Client Secret; overrides `IGDB_CLIENT_SECRET` and the saved one |
| `-bhl-key`     | (none)   | Biodiversity Heritage Library API key, for nature; overrides `BHL_API_KEY` and the saved key |

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

## Language

The app is available in English and Spanish.

- By default it follows the system language: Spanish when the system language is Spanish (any region), English otherwise.
- To choose a language, open **Settings** and set **Language** to **System**, **English** or **Español**. The change applies at once and is saved in the settings file (`"language"`: `""` for System, `"en"` or `"es"`).
- Translated: the interface, native dialogs, error messages, item statuses, and the labels printed in PDFs (review card date label, ticket stub labels).
- Not translated: technical details from the sources (for example `HTTP 503`), and the `covers` command-line tool, which stays in English.

To add a language, add a dictionary next to `frontend/src/i18n/en.ts` (the compiler rejects missing or extra keys) and a catalog in `internal/i18n/catalog.go` (a test checks that every key exists).

## Development

```sh
wails dev -tags webkit2_41        # Linux; on Windows just: wails dev
```

Run the frontend unit tests with `npm test` in `frontend/` (vitest), and the Go tests with `go test ./...`.

This runs the app with hot reload of the frontend. Wails regenerates the TypeScript bindings in `frontend/wailsjs/` when Go method signatures change (or run `wails generate module -tags webkit2_41`).

## Building

```sh
# Linux binary: build/bin/bujo-helper
wails build -tags webkit2_41

# Windows binary: build/bin/bujo-helper.exe
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
  providers/           books: Apple Books, Open Library, Amazon; movies: TMDB, IMDb, Wikidata → iTunes; TV: TMDB, IMDb; albums: MusicBrainz + Cover Art Archive, iTunes; games: IGDB, Steam; anime and manga: AniList; artworks: the Met, Cleveland; places: Wikidata → Wikimedia Commons; nature: Wikidata taxa → Wikimedia Commons
  match/               title/creator matching and ranking
  batch/               text lines and CSV parsing
  downloader/          concurrent batch downloads, local-edition preference, image validation, file naming, credit files
  locale/              country table: language and ISBN prefixes of local editions
  isbn/                ISBN-10/13 validation, conversion and extraction
  web/                 shared HTTP client, rate-limit throttle, headers and bounded reads
  settings/            user settings file (TMDB API key)
  gallery/             print sheet: folder listing and thumbnails for the UI
  sheet/               print sheet: layouts (covers, review cards, spines, ticket stubs, full page, checklist, polaroid) and PDF export
build/                 Wails build assets (icons, manifests); binaries go to build/bin/
legacy/                the original Python script
```

## Adding a new media type

1. Implement `media.Provider` (`Name()` and `Search(ctx, item, opts)`) for each source, usually in `internal/providers`. Return candidates best-first with their image URLs ordered best-first. Return an error wrapping `media.ErrNotFound` when there is no match. Set `Credit` on a candidate whose image needs an attribution file; the downloader saves it as `<image file>.credit.txt`.
2. Register a `media.MediaType` in `internal/catalog` with an ID, a label, a creator label (for example `Year` or `Artist`) and its providers in fallback order. Set `LocalEditions` only for types whose candidates carry an ISBN (books).

That's all. The UI lists media types from the registry and uses the creator label in its hints, so no frontend changes are needed.

## Legacy

`legacy/cover_downloader.py` is the original single-title Python script that this app replaces. It is kept for reference only and is not part of the build.

## License

[MIT](LICENSE)
