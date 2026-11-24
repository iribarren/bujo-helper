# BuJo Helper

Formerly "Cover Downloader".

A desktop app for Windows, macOS and Linux that downloads cover images in batches.
Paste a list of `Title, Author` lines, or import a CSV file, and it saves one cover image per item to a folder.

Book covers, movie posters, TV series posters, album covers, podcast artwork, game box art, anime and manga covers and board game box art are supported. The core is media-agnostic, so other media can be added later as new providers.

## Features

- Batch input: one `Title, Author` (books), `Title, Year` (movies, TV series, games, anime and manga, board games), `Album, Artist` (albums), `Podcast, Host` (podcasts), `Title, Artist` (artworks), `Place, Country` (places), a plant or animal `Name` (nature) or `Product, Brand` or a barcode (products) per line, or import a CSV file (the import replaces the list).
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
- Food and drink product pictures (tea, coffee, wine, beer, snacks) from Open Food Facts, by name and brand or by barcode, each saved with a credit file, see [Products](#products).
- Board game box art from BoardGameGeek, each saved with a credit file, see [Board games](#board-games).
- Store country selection (default `es`; for example `us`, `gb`, `de`). Covers of that country's editions are preferred, see [Local editions](#local-editions).
- **Open folder** shows the results in the system file manager.
- **Print sheet**: select downloaded covers and export them as an A4 PDF ready to cut out, with covers 2.5, 5 or 8 cm tall (81, 25 or 9 per page), with a grey cut border or a 1 mm bleed and crop marks for print-then-cut, with your own photos centre-cropped to standard print shapes (instant mini, square, 2 × 6 in photo strips or wallet), as review cards with a date, stars and lines to fill in, as book spines for a bookshelf tracker, as movie ticket stubs, as a full page of covers under a header such as "Books of 2026", sized to fit and ready to glue in, as a checklist (TBR list or watchlist) with a checkbox, thumbnail, title and creator per row, or as polaroid frames with the title as a caption. A live preview shows the PDF before you export it. See [Print sheet](#print-sheet).
- **Lettering**: headers and titles print in one of ten bundled lettering fonts (brush and monoline script, handwritten, bubble display, hollow), optionally as outlines to colour in by hand. See [Lettering](#lettering).
- Light and dark themes: they follow the system setting, or pick one in **Settings**, see [Theme and sections shown](#theme-and-sections-shown).
- Hide what you don't use: choose in **Settings** which sections, media types, print formats, papers and page kinds the app shows.
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

Podcasts have one source: **Apple Podcasts** (iTunes Search API, podcast search, no key). Write `Podcast, Host` (`The Daily, The New York Times`); the host is the name the show is published under in Apple Podcasts, and it is optional, but it helps when several shows share a title. The show's artwork comes at its original size (usually 3000 × 3000). It uses the store country. Podcast artwork is square, so a cover sheet gives it square cells, like album covers. There is no keyless fallback: the show's RSS feed is found through the same iTunes lookup, and Podcast Index needs a key. Apple's terms are the same as for the Apple Books and Apple Music sources above; the artwork belongs to each show, and downloads are for personal use.

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

Place, nature, product and board game images get one too (see [Places](#places), [Nature](#nature), [Products](#products) and [Board games](#board-games)); other media types get no credit file. The print sheet can list them on a [credits page](#credits-page). The print sheet gallery lists images only, never credit files. The app never deletes or renames downloaded images; if you do, do the same to the credit file.

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

### Products

Products download the front picture of a food or drink product (tea, coffee, wine, beer, snacks) from [Open Food Facts](https://world.openfoodfacts.org/) (no key). Write `Product, Brand` (`Nutella, Ferrero`, `Earl Grey, Twinings`); the brand is optional, but it helps. A line of 8 to 14 digits with no brand is a barcode (`3017620422003`); spaces and hyphens in it are ignored (`3 017620 422003`). Product pictures come in every shape, so a cover sheet gives landscape ones wide cells (see [Print sheet](#print-sheet)).

1. A **barcode** is looked up directly with the product API (`world.openfoodfacts.org/api/v2/product/<barcode>`). An unknown barcode fails with "no Open Food Facts product with barcode …".
2. Otherwise **Search-a-licious** (`search.openfoodfacts.org`) searches the name, and the brand when given, up to 10 results. The closest names come first, and up to three products with a usable picture are kept.
3. Each product's front picture is chosen in this order: the store country's language (`es` gives Spanish), the product's own language, English, then any other front picture.
4. The full-size picture is used only when its shorter side is at least 400 px (a 50 mm cover at about 200 dpi). Smaller pictures, and pictures of unknown size, are skipped; when no product is left the item fails and names the skipped products with the reason, for example `Nutella (3017620422003): front image too small (300×200 px)`.

A barcode line saves as `<barcode>.jpg`, since file names come from what you typed. Write `Product, Brand` to get a named file.

The legacy text search (`/cgi/search.pl`) answered "Page temporarily unavailable" when checked (2026-11-23), and the v2 product API has no text search, so name searches use Search-a-licious.

**Licence and rights.** Open Food Facts product images are under [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/). Its [terms of use](https://world.openfoodfacts.org/terms-of-use) allow reuse for any purpose, as long as you name the licence and credit Open Food Facts with a link to the product page; contributors are credited through that same link. Share-alike applies to derivative works: if you publish an edited version of an image, it must stay under CC BY-SA. Product packaging shows brand designs and trademarks that the licence does not cover, and Open Food Facts leaves those to you to check: print them for your own journal. Open Food Facts asks that one API call match one real user request; the app only looks up the items you ask for, one at a time, and never scrapes in bulk.

**Credit file.** Each product image is saved with a credit file next to it, the same way as places: the product name and brand, the contributor who uploaded the picture (when known), the source, the licence and the product page. For example, for `Nutella, Ferrero`:

```text
Title: Nutella (Ferrero)
Contributor: kiliweb
Source: Open Food Facts
License: CC BY-SA 3.0 (https://creativecommons.org/licenses/by-sa/3.0/)
Product page: https://world.openfoodfacts.org/product/3017620422003
```

When you print or share a page with these images, credit each one; the print sheet's [Credits page](#credits-page) does this for you.

Open Food Facts asks every client for a User-Agent that names the app and a contact. The app sends `bujo-helper/1.0 ( https://github.com/iribarren/bujo-helper )` to its APIs and its image server. It allows 15 product reads and 10 searches per minute per IP, and going over can get the IP banned, so the app spaces product reads 4.5 s apart and searches 6.5 s apart, a margin under both. A long product list takes several seconds per item.

### Board games

Board games download box art from [BoardGameGeek](https://boardgamegeek.com/) (BGG), through its XML API2. Write `Title, Year` (`Catan, 1995`, `Wingspan, 2019`); the year is the first publication year, and it is optional. BGG needs an application token, which release builds include, see [BoardGameGeek token](#boardgamegeek-token). Without a token the source is skipped with "no BoardGameGeek token in this build (add one in Settings)". Box art comes in every shape, so a cover sheet gives square boxes square cells and landscape ones wide cells (see [Print sheet](#print-sheet)).

1. **Search** (`/xmlapi2/search?type=boardgame`) for the exact title (`exact=1`), then, when that finds nothing, again without `exact`. Only board games are kept: expansions and accessories are ignored. A title with no match fails with "no BoardGameGeek board game matches …".
2. The results are ranked by title (the same title first, then one that starts with the other, then one that contains it) and year (the year asked for, then one off by one, then an unknown year). The best 20 are read in one **thing** request (`/xmlapi2/thing?id=…&stats=1`) for their names, year, image and number of ratings. Primary and alternate names both match, so `Die Siedler von Catan` also finds `CATAN`.
3. They are ranked again by title, then year, then number of ratings (the most rated first, so among several games called `Dune` with no year asked for, the most rated one wins). Up to three with box art are kept, best first; a game without an image is skipped. When none has one, the item fails with "no BoardGameGeek board game with box art matches …".

A token BGG rejects fails the item with "BoardGameGeek rejected the token (HTTP 401); check the BoardGameGeek token in Settings" (or HTTP 403). BGG answers 500 or 503 when it throttles; the app reports "BoardGameGeek is busy (HTTP 503); try again later" (also for HTTP 429 and 500).

**Licence and rights.** The [XML API Terms of Use](https://boardgamegeek.com/wiki/page/XML_API_Terms_of_Use) grant a non-commercial licence, free of charge, that BGG can revoke at any time. They require crediting BoardGameGeek by name wherever its data is used and showing the "Powered by BGG" logo, linked to BGG, in public-facing uses: the app shows it in the Download tab while **Board games** is selected and in **Settings → About**. They forbid modifying the data and using it to train AI or large language models. See also BGG's guide [Using the XML API](https://boardgamegeek.com/using_the_xml_api). Box art is copyrighted by its publishers, like book covers: downloaded images are for personal use only. This app is not endorsed by BoardGameGeek.

**Credit file.** Each board game image is saved with a credit file next to it, the same way as places: the game and its year, the source, the game page and the copyright note. For example, for `Catan, 1995`:

```text
Title: CATAN (1995)
Image: BoardGameGeek
Game page: https://boardgamegeek.com/boardgame/13
Box art © its publisher; personal use only.
```

The title is BGG's primary name for the game. The print sheet's [Credits page](#credits-page) lists these credits.

BGG publishes no rate limit, but says a 5-second wait between requests seems to suffice and asks apps to keep requests few. The app sends `bujo-helper/1.0 ( https://github.com/iribarren/bujo-helper )` and the token (`Authorization: Bearer …`) on every request to `boardgamegeek.com`, and spaces those requests 5 s apart. A board game makes two or three requests (a second search only when the exact one finds nothing), so a long list takes 10 to 15 s per game.

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

### BoardGameGeek token

BoardGameGeek's XML API needs a registered application and its token on every request. BGG asks apps for end users not to make them get their own token, so release builds include the app's own token: board games work with nothing to set up.

- Builds from source have no built-in token (see [Building](#building)). While **Board games** is selected and the build has no token and none is saved, a tip above the list suggests adding one.
- A token saved in **Settings → BoardGameGeek token** overrides the built-in one. It is saved next to the TMDB key, readable only by you, and sent only to BoardGameGeek.
- For the command line, use `-bgg-token`, or the `BGG_TOKEN` environment variable; otherwise the token saved by the app is used, and then the built-in one.

To get a token (for a source build or a fork):

1. Sign in at [boardgamegeek.com](https://boardgamegeek.com/) and register a non-commercial application at [boardgamegeek.com/applications](https://boardgamegeek.com/applications). BGG approves applications by hand, which may take a week or more.
2. Once it is approved, create a token for the application on the same page.
3. Paste it into **Settings**, or build it in (see [Building](#building)).

### Rate limits

All sources share one HTTP client that respects rate limits. A `429 Too Many Requests` answer (or a `503` with `Retry-After`) pauses that source for every download: the client waits for the `Retry-After` time (or 1 s, 2 s, 4 s if none is given) and retries up to 3 times. A wait longer than 60 s is not attempted; the item falls back to the next source instead. Requests to TMDB, Wikidata and the Commons API are also spaced out slightly, requests to IGDB 250 ms apart (IGDB allows four per second), requests to AniList 2 s apart (AniList allows 30 per minute while degraded), requests to MusicBrainz one second apart, requests to the Met 25 ms apart, image downloads from Wikimedia Commons 500 ms apart, requests to the BHL API and its page scans 500 ms apart (BHL has IP rate limits but publishes no numbers), and Open Food Facts product reads (`world.openfoodfacts.org`) 4.5 s apart, searches (`search.openfoodfacts.org`) 6.5 s apart and image downloads (`images.openfoodfacts.org`) 500 ms apart (Open Food Facts allows 15 product reads and 10 searches per minute), and requests to BoardGameGeek (`boardgamegeek.com`) 5 s apart (BGG publishes no limit, answers 500 or 503 when it throttles, and says a 5-second wait suffices). A nature item makes at most three Wikidata requests (once per name) and two Commons API requests per search, at most three searches; with a BHL key, BHL adds one name request and up to three volume requests. IGDB and AniList searches are POST requests; they are retried like the others, since a rate-limited request was not processed.

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

- One item per row: `title,author` for books, `title,year` for movies, TV series, games, anime and manga, `album,artist` for albums, `title,host` for podcasts, `title,artist` for artworks, `place,country` for places, a single `name` column for nature, `title,brand` for products (a barcode with no brand also works), and `title,year` for board games. The second column is optional and extra columns are ignored.
- An optional header row (for example `title,author`) is detected and skipped.
- Comma (`,`) or semicolon (`;`) delimiters are detected automatically.
- UTF-8 (with or without BOM) and Windows-1252 (the Excel default on Windows) are both accepted.
- Quote titles that contain the delimiter: `"Guns, Germs, and Steel",Jared Diamond`.

See [`sample_books.csv`](sample_books.csv) for an example. In the app, the **Sample CSV** link next to **Import CSV…** saves an example file for the selected media type (books, movies, TV series, albums, games, anime, manga, artworks, places, nature, products, board games or podcasts).

In the text box, a line is split on its **last** comma, so `Guns, Germs, and Steel, Jared Diamond` works without quotes. Blank lines and lines starting with `#` are ignored.

## Using the app

1. Choose the media type, such as **Books**, **Movies**, **TV series**, **Albums**, **Podcasts** or **Games**. For movies and TV series, optionally add a TMDB API key in **Settings** for the best posters; for games, add IGDB credentials for box art beyond PC games; for nature, a BHL API key adds plates when Commons has none. The line under the media type buttons lists the sources tried for that type, in order (for books: Apple Books, Open Library, Amazon).
2. Type or paste items, one per line, or click **Import CSV…**.
3. Check the output folder (default: `~/Pictures/Covers`, or `%USERPROFILE%\Pictures\Covers` on Windows), and change it with **Browse…** if needed.
4. Optionally change the two-letter store country (default `es`, which prefers Spanish editions).
5. Click **Download covers** or press **Ctrl+Enter**.
6. When items finish, click **Open folder** to see the images, or **Print these covers** to switch to the [print sheet](#print-sheet) tab on the folder the covers were saved to, even if you chose another folder there before.

## Print sheet

The **Print sheet** tab turns downloaded covers into a PDF to cut out, for example for a bullet journal, on A4, US Letter, A5 or B5 paper: a sheet of small covers (or of photos [cropped](#crop-to) to standard print shapes), [review cards](#review-cards), [spines](#spines), [ticket stubs](#ticket-stubs), a [full page](#full-page) of covers under a header, with nothing to cut, a [checklist](#checklist) to tick off as you read or watch, or [polaroids](#polaroid) with the title as a caption.

1. The folder defaults to the download output folder and follows it until you pick another one with **Browse…**. The covers load when the tab opens; **Refresh** reloads them.
2. Pick a **Format**: **Cover sheet** (the default), **Review cards**, **Spines**, **Ticket stubs**, **Full page**, **Checklist** or **Polaroid**. For review cards, pick what they are **Cards for** (Books, Movies, TV series or Albums); it follows the Download tab's media type until you change it here. For a full page or a checklist, type a **Header** under **Lettering** (for example "Books of 2026" or "TBR 2026"), or leave it empty; for the other formats, Lettering says the format prints no header. For a full page, a checklist or review cards, Lettering also has the **Font** and **Colour-in** controls (see [Lettering](#lettering)).
3. Pick a **Paper** (buttons side by side): **A4** (the default), **US Letter**, **A5** or **B5** (176 × 250 mm, the size of many bullet journal notebooks). Each **Cover size** option shows how many covers (or spines, or polaroids) fit on that paper. On a cover sheet it follows the selection: when every selected image is square (albums), how many square covers fit; when every one gets a wide cell at that size (landscape artworks), how many images of the selection's middle shape fit, which is exact when they all share one shape; otherwise, and for mixed shapes, how many 2:3 covers fit.
4. Pick a **Cover size**: **Mini**, **Standard** (the default) or **Large**. It applies to the cover sheet, spines and polaroids, so it is disabled for review cards, ticket stubs, full pages and checklists. On a cover sheet it is hidden for the crop modes with a fixed size (Instant mini, Photo strip and Wallet).
5. For a cover sheet, pick what to **Crop to**: **None** (the default, whole images), **Instant mini**, **Square**, **Photo strip (3 frames)**, **Photo strip (4 frames)** or **Wallet** (see [Crop to](#crop-to)), and a **Cut mode**: **Border** (the default), **Bleed + crop marks** or **None** (see [Cut mode](#cut-mode)). The cut mode never changes how many covers fit on a page; it is hidden for Instant mini and photo strips, which always keep their frame border.
6. Leave **Credits page** checked (the default) to end the PDF with the author and licence of every selected image that has a credit file, in every format (see [Credits page](#credits-page)). Uncheck it to print the images only.
7. Click covers (or focus one and press Space or Enter) to select them. **Select all** and **Select none** help with large folders. The toolbar shows how many covers are selected and how many pages they need with the chosen format, paper and size (and, for a checklist, header), including any credits pages. The page count is exact: it reads the size of each selected image, since square and landscape covers take more room on a cover sheet (an image that cannot be read counts as a 2:3 cover).
8. To see the PDF before saving it, switch the right-hand pane from **Gallery** to **Preview** (the switch is at the start of the toolbar; arrow keys move between the two). It shows the PDF of the selected covers with the current options, one page at a time with **‹** and **›** and "Page X of N", and updates about 300 ms after the last change to the selection or an option. It is the same PDF **Export PDF…** writes. Nothing is rendered while the gallery is shown.
9. Click **Export PDF…**, choose where to save (the suggested name gives the format, paper, cover size and any cut mode other than Border, for example `covers_A4_Standard.pdf`, `covers_Letter_Mini.pdf`, `covers_A4_Standard_bleed.pdf`, `covers_A4_instax.pdf`, `covers_A4_Large_square.pdf`, `spines_A4_Standard.pdf`, `polaroid_A4_Standard.pdf` or, for review cards, ticket stubs, full pages and checklists, `cards_A4.pdf`, `stubs_A4.pdf`, `fullpage_A4.pdf` and `checklist_A4.pdf`), then **Open PDF** to view it.

Covers are placed in the grid order (file name, case-insensitive, numbers in natural order). JPEG, PNG, GIF and WebP images are supported.

The tab remembers its options between runs: the format, what review cards are for (only once you picked it here), the cover size, crop mode, paper, cut mode, credits page, header, font and Colour-in. They are saved in the settings file (under `"print"`, see [TMDB API key](#tmdb-api-key) for where it lives) each time you change one, the header once you leave the field. An option the app no longer offers falls back to its default. The folder and the Gallery | Preview switch are not remembered.

Grid per paper and cover size (columns × rows = covers per page):

| Paper     | Page (mm)     | Mini (25 mm) | **Standard** (50 mm) | Large (80 mm) |
|-----------|---------------|--------------|----------------------|---------------|
| **A4**    | 210 × 297     | 9 × 9 = 81   | 5 × 5 = 25           | 3 × 3 = 9     |
| US Letter | 215.9 × 279.4 | 9 × 9 = 81   | 5 × 4 = 20           | 3 × 3 = 9     |
| A5        | 148 × 210     | 6 × 6 = 36   | 3 × 3 = 9            | 2 × 2 = 4     |
| B5        | 176 × 250     | 7 × 8 = 56   | 4 × 4 = 16           | 2 × 2 = 4     |

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

- **Square cells:** a square cover (width / height within about 10 % of 1, such as album art or podcast artwork) gets a square cell, *h* × *h*, instead of a column. Cells flow left to right, 4 mm apart, and a row wraps when the next cell does not fit, so a folder of books and albums mixes both kinds of cell. The rows per page stay the same. Each row is centred, and a short row is centred as if it were filled with more cells like its last one, so the last row lines up with the full rows above it. A sheet whose covers all fit the 2:3 column keeps the grid in the table above.
- **Wide cells:** a cover wider than the column at full height that is not square gets a wide cell, as wide as the cover at height *h*: a 3:2 painting at Standard size gets a 75 × 50 mm cell, a 4:5 portrait 40 × 50 mm. Wide cells flow, wrap and centre like square cells. No cell is wider than the space between the side margins: a panorama wider than that shrinks to fit, so it is shorter than *h* and centred in its row (a 5:1 panorama on A4 Standard is 194 × 38.8 mm). Whether a 4:5 portrait needs a wide cell depends on the column: on A5 Standard and Large it fits the column.
- **3:2 covers per page:** A4 4 × 9 = 36 (Mini), 2 × 5 = 10 (Standard), 1 × 3 = 3 (Large); US Letter 4 × 9 = 36, 2 × 4 = 8, 1 × 3 = 3; A5 3 × 6 = 18, 1 × 3 = 3, 1 × 2 = 2; B5 3 × 8 = 24, 2 × 4 = 8, 1 × 2 = 2.
- **Square covers per page:** A4 6 × 9 = 54 (Mini), 3 × 5 = 15 (Standard), 2 × 3 = 6 (Large); US Letter 7 × 9 = 63, 3 × 4 = 12, 2 × 3 = 6; A5 4 × 6 = 24, 2 × 3 = 6, 1 × 2 = 2; B5 5 × 8 = 40, 3 × 4 = 12, 1 × 2 = 2.
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

### Crop to

**Crop to** prints the photos of a cover sheet in a standard print shape, for example your own photos on sticker paper. Each photo is centre-cropped to fill its window exactly, with no distortion: a photo wider than the window loses equal strips at the left and right, a taller one at the top and bottom. The image files are never changed; cropping happens when the PDF is built, before the usual 300 dpi downscale.

| Crop to                | Printed piece (mm)                       | Photo window (mm)                                       | Size / cut mode |
|------------------------|------------------------------------------|---------------------------------------------------------|-----------------|
| **None** (default)     | the cover sheet above                    | whole images, not cropped                               | both apply      |
| Instant mini           | 54 × 86 white frame, grey cut border     | 46 × 62, 4 mm from the sides and 7 mm from the top (17 mm empty band below) | neither         |
| Square                 | *h* × *h*, filled by the photo           | the whole piece; *h* is the **Cover size** (25, 50 or 80 mm) | both apply |
| Photo strip (3 frames) | 50.8 × 152.4 (2 × 6 in) strip, grey cut border | 3 frames of 44.8 × 46.8, 3 mm from the edges and apart | neither  |
| Photo strip (4 frames) | 50.8 × 152.4 (2 × 6 in) strip, grey cut border | 4 frames of 44.8 × 34.35, 3 mm from the edges and apart | neither |
| Wallet                 | 64 × 89, filled by the photo             | the whole piece                                         | cut mode only   |

- A photo strip takes consecutive photos, top to bottom; the last strip may have empty frames.
- Square and wallet photos follow the [cut mode](#cut-mode) like covers, bleed and crop marks included (the bleed repeats the cropped photo's edge pixels).
- Pieces are centred on the page with 4 mm gaps and at least 8 mm margins. Photos per page (pieces for strips × their frames):

| Paper     | Instant mini | Square Mini / **Standard** / Large | Photo strip (3) | Photo strip (4) | Wallet    |
|-----------|--------------|------------------------------------|-----------------|-----------------|-----------|
| **A4**    | 3 × 3 = 9    | 54 / 15 / 6                        | 3 strips = 9    | 3 strips = 12   | 2 × 3 = 6 |
| US Letter | 3 × 2 = 6    | 63 / 12 / 6                        | 3 strips = 9    | 3 strips = 12   | 2 × 2 = 4 |
| A5        | 2 × 2 = 4    | 24 / 6 / 2                         | 2 strips = 6    | 2 strips = 8    | 2 × 2 = 4 |
| B5        | 2 × 2 = 4    | 40 / 12 / 2                        | 2 strips = 6    | 2 strips = 8    | 2 × 2 = 4 |

- The page count and size labels count photos, whatever their shape.
- The suggested file name adds the crop mode after the cover size (only for Square), for example `covers_A4_instax.pdf`, `covers_A4_Large_square.pdf`, `covers_Letter_wallet_bleed.pdf` or `covers_A4_strip4.pdf`.
- Only the cover sheet crops; the other formats ignore **Crop to**.

### Credits page

Wikimedia Commons images (see [Places](#places) and [Nature](#nature)) are often CC BY or CC BY-SA, and Open Food Facts images (see [Products](#products)) are CC BY-SA, which require credit and the licence wherever the image is reproduced, printed sheets included; artworks are public domain, but credit is still good practice. With **Credits page** checked (the default), the PDF ends with one or more pages titled "Credits" ("Créditos" in Spanish), on the same paper, listing every selected image that has a credit file (`<image>.credit.txt`, see [Artworks](#artworks), [Places](#places), [Nature](#nature), [Products](#products) and [Board games](#board-games)): a number, the image's title and creator from its file name, and the credit text, wrapped to the page width, in the order the images appear in the sheet.

- Images without a credit file (books, movies, TV series, albums, podcasts, games, anime, manga) are not listed. When no selected image has one, or the box is unchecked, no page is added and the PDF is exactly what it would be without the option.
- A credit file that exists but cannot be read stops the export, naming the file, like an unreadable image.
- Credits print in the embedded Go font, which covers Latin, Greek and Cyrillic text; characters it lacks (for example Japanese) print as empty boxes, though they can still be copied from the PDF, and emoji print as "?".
- When the header or card titles print in a bundled [lettering font](#lettering), the credits end with a **Fonts** section ("Fuentes" in Spanish) naming that font and its licence. It is added only to a credits page that prints anyway: a font never adds a page on its own.

### Review cards

The **Review cards** format prints one 90 × 55 mm card per selected cover, to fill in after reading or watching:

| Part       | Content                                                                                       |
|------------|-----------------------------------------------------------------------------------------------|
| Left side  | the cover                                                                                     |
| Title      | from the file name `Title - Creator.jpg`, up to 2 lines; longer text ends with "…"; in the chosen [lettering font](#lettering), always filled |
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
| B5        | 1 × 4          | 4              |

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
| B5        | 13 × 8 = 104   | 13 × 4 = 52          | 13 × 2 = 26   |

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
| B5        | 1 × 5          | 5              |

- The title and creator are read from the file name as for [review cards](#review-cards), with the same caveats.
- The suggested file name is `stubs_<paper>.pdf`, for example `stubs_A4.pdf`.

### Full page

The **Full page** format prints a finished page to glue or bind into a journal, with nothing to cut: the **Header** you type (for example "Books of 2026" or "Watched in March"), then the selected covers in a tight grid that fills the page. There are no cut borders.

The cover size is chosen automatically, so up to 60 covers fit on one page:

| Setting      | Rule                                                                                                     |
|--------------|----------------------------------------------------------------------------------------------------------|
| Margins      | at least 8 mm on every side                                                                              |
| Header       | bold, centred, in a 12 mm band right above the covers; 20 pt, shrunk down to 12 pt to fit the width, then cut with "…". Empty: no band, the covers use the whole height. Font and outlines: see [Lettering](#lettering) |
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
| Header    | as on a [full page](#full-page): bold, centred, in a 12 mm band, shrunk then cut with "…" to fit, in the chosen [lettering font](#lettering); repeated on every page. Empty: no band |
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
| B5        | 14            | 13            |

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
| B5        | 6 × 6 = 36 | 3 × 3 = 9    | 2 × 2 = 4 |

- The title is read from the file name as for [review cards](#review-cards), with the same caveats.
- The paper and cover size apply; the cut mode and header do not (the border always stays).
- The suggested file name is `polaroid_<paper>_<size>.pdf`, for example `polaroid_A4_Standard.pdf`.

## Generated pages

Some printables need no downloaded images. Each one is a **page kind** with its own options, on any paper, built by `internal/sheet` (`PageKinds`, `BuildPage`) and exported through the `PageKinds`, `PagePages`, `PreviewPage` and `ExportPage` bindings. The suggested file name gives the kind and the paper, for example `dotgrid_A5.pdf`.

They live in the **Make pages** tab, in three columns:

- **Page**: the page kinds; pick one.
- **Options**: the kind's own options, then **Page** (the paper, and the cut mode for kinds that are cut out) and **Lettering** (the title, for kinds that print one, and the **Font** and **Colour-in** controls, see [Lettering](#lettering)).
- **Preview**: a live preview of the PDF, shown by default. It updates about 300 ms after the last option change and stays on the page shown (or the last one, if the PDF gets shorter); **‹** and **›** step through the pages, with "Page X of N". It is the same PDF **Export PDF…** writes. **Export PDF…** saves it, starting in the Print covers folder, and **Open PDF** opens the saved file.

The preview is rendered by Go into memory (no temporary file) and served by the app's asset handler at `/preview`, which keeps only the last few renders; the frontend draws it onto a `<canvas>` with [pdf.js](#third-party-software), the same way on every platform. The same handler serves the cover thumbnails at `/thumbs` and the [lettering fonts](#lettering) at `/fonts/`.

The tab remembers the last page kind and the shared options (paper, cut mode, title, font and Colour-in) between runs, in the settings file under `"pages"`, separately from Print covers; a kind's own options (spacing, month, habits…) are not remembered.

To make a dot grid page: open **Make pages**, pick **Dot grid**, choose the dot spacing and the paper, type a title if you want one, and click **Export PDF…**.

### Dot grid

A plain grid of small grey dots, the basic bullet journal page, on one page.

| Setting | Rule                                                                                                   |
|---------|--------------------------------------------------------------------------------------------------------|
| Spacing | 4, **5** or 6 mm between dot centres                                                                   |
| Dots    | 0.25 mm radius, mid grey (#808080)                                                                     |
| Area    | as many dots as fit a whole number of spacings apart within 8 mm margins, centred both ways            |
| Title   | optional, bold, in a 12 mm band at the top margin like a [full page](#full-page) header, in the chosen [lettering font](#lettering); the dots start below it |

On A4 at 5 mm that is 39 × 57 dots (39 × 54 with a title). The cut mode does not apply.

### Moon calendar

A month or a whole year of moon phases, one moon per day, with the new and full moons marked. The phases are computed on your computer: no network, no API key, nothing to license.

To make one: open **Make pages**, pick **Moon calendar**, choose the month, year, layout and hemisphere, and click **Export PDF…**.

| Setting      | Rule |
|--------------|------|
| Month        | January to December; starts on the current month |
| Year         | 1900 to 2100; starts on the current year. On leaving the box, a whole year out of range is clamped (1850 → 1900) and a partial one (`20`) goes back to the last valid year |
| Layout: Row  | one line per day (day number and weekday, moon, mark), in two columns: 1–16 and 17 to the end |
| Layout: Grid | the default: a month grid under weekday headers; day number top left, moon centred, mark below |
| Layout: Year | 12 month rows of days under a day-number header; Month is hidden and marks shrink to their initial (N/F, in Spanish N/L) |
| Hemisphere   | **North** (default) or **South**, which mirrors each moon left to right, as it looks from the southern hemisphere |
| Title        | the shared **Title**, in the chosen [lettering font](#lettering); empty prints the month and year ("March 2024"), or the year ("2024") for the Year layout |
| Labels       | month and weekday names, day numbers and New/Full marks follow the UI language, in the Go font |
| Moons        | a dark disc (#3b4252), the lit part white, a thin outline; with [Colour-in](#colour-in), outlines only |
| Weeks        | start on Monday |
| Area         | within 8 mm margins, below a 12 mm title band |

How the phases are computed:

- New and full moon times come from Jean Meeus, *Astronomical Algorithms* (2nd ed.), chapter 49 (true phases with their periodic corrections), converted to UTC with the Espenak–Meeus ΔT polynomials.
- The moon drawn on each day shows the fraction of the lunation (new moon to next new moon) elapsed at local noon.
- Checked against the USNO phase table for 2024: every new and full moon within one minute.
- Days and marks use the computer's local time zone. A new or full moon near midnight is marked on the local date it falls on, which can differ from a table in UTC.

The cut mode does not apply.

### Month calendar

A month grid with a box per day to write in, for any month from 1900 to 2100, weeks starting on Monday or Sunday. It fits one page.

To make one: open **Make pages**, pick **Month calendar**, choose the month, year and week start, and click **Export PDF…**.

| Setting        | Rule |
|----------------|------|
| Month          | January to December; starts on the current month |
| Year           | 1900 to 2100; starts on the current year, with the same typing rules as the [moon calendar](#moon-calendar) |
| Week starts on | **Monday** (default) or Sunday: the weekday in the first column |
| Title          | the shared **Title**, in the chosen [lettering font](#lettering), outlined with [Colour-in](#colour-in); empty prints the month and year ("March 2024", in Spanish "Marzo 2024") |
| Labels         | short weekday names (Mon, Tue, …; in Spanish Lun, Mar, …) and day numbers follow the UI language, in the Go font |
| Paper          | any; within 8 mm margins, below a 12 mm title band |

How the grid is drawn:

- 7 columns as wide as the area, under an 8 pt weekday header row, and as many week rows as the month spans (4 to 6).
- Rows share the height below the header, but a cell is never taller than 1.5 times its width; on tall paper the grid is then centred down the page.
- Each day is a box with a thin grey border (0.2 mm) and its number at the top left, 9 pt. The empty cells before the first day and after the last have no box.
- Dates are checked for leap years (2024, 2000, 1900 and 2100) and both week starts.

The cut mode does not apply.

### Habit tracker

A habit per row and a box per day of the month, to tick off each day you keep it. With no habits it prints blank rows to write them in by hand.

To make one: open **Make pages**, pick **Habit tracker**, choose the month and year, type your habits one per line, and click **Export PDF…**.

| Setting | Rule |
|---------|------|
| Month, Year | as on the [month calendar](#month-calendar); the month sets the number of day columns (28 to 31) |
| Habits, one per line | up to 40 habits of up to 60 characters each; spaces at either end are trimmed and blank lines are dropped. More is an error, shown in the preview |
| Title | the shared **Title**, in the chosen [lettering font](#lettering), outlined with [Colour-in](#colour-in); empty prints "Habits ·" and the month ("Habits · November", in Spanish "Hábitos · Noviembre") |
| Paper | any; within 8 mm margins, below a 12 mm title band |

How the tracker is drawn:

| Part | Rule |
|------|------|
| Day header | the day numbers over their columns, 6 pt, shrinking to 4 pt to fit narrow boxes |
| Name column | as wide as the longest habit name, between 20 mm and 30 % of the width (30 % with no habits); it also takes any width the boxes leave. Names print at 7 pt, in the Go font; a name that does not fit ends with "…" |
| Boxes | square, sharing the rest of the width, at most 6 mm |
| Rows | 1.5 times the box size, at least 6 mm, from the top of the area down |
| No habits | one page filled with blank rows, each a writing line in the name column and a box per day |
| More rows than fit | the rest go on further pages, each with the title and the day header. with 40 habits, two pages on every paper; the page count follows the paper |

Habit names are cleaned before they are counted and printed: each run of tabs, line breaks, other invisible control characters or spaces becomes one space, so `Read`, a tab and `books` print as "Read books", and a habit of only such characters is dropped like a blank line. A list pasted from a spreadsheet or another app can carry characters you cannot see; turning them into a space is kinder than an error that names an invisible character.

The cut mode does not apply.

### Quote cards

Quotes laid out as cut-out cards, two columns of them, in the lettering font. Search a person on Wikiquote and tick the quotes you want, type your own, or both. With no quotes it prints a page of empty card frames to write in by hand.

To make one: open **Make pages**, pick **Quote cards**, type a name in **Find quotes by** (for example `Mark Twain`), click **Search** (or press Enter), tick the quotes you want, and click **Export PDF…**.

| Setting | Rule |
|---------|------|
| Find quotes by | an author or personality, searched on [Wikiquote](#quotes-from-wikiquote) in the UI language (English or Spanish). The person found heads the results, with **Other matches** below it: click one to search that page instead |
| Quotes found | the person's quotes as a checklist, with **All** · **None** and the number ticked. Ticked quotes go to **Picked quotes**, which keeps them across searches in the order you picked them (the same quote and author count once); **×** removes one |
| Your own quotes: Quote, Author | one quote per line; the author goes after the last comma, so `Less is more, Mies van der Rohe` works and a comma inside the quote is kept. A line without a comma is a quote with no author; blank lines are dropped. To keep a comma in the author, or in a quote with no author, put that part in double quotes, as in CSV (`""` inside is one `"`): `Free at last, "Martin Luther King, Jr."`. A line is read this way only when its quotes are well-formed; any other line splits at the last comma as above. A quote typed in quotes, `"Be yourself", Oscar Wilde`, prints without them (Classic and Framed add their own) |
| Style | **Classic** (default): centred, in curly quotes, the author centred below. **Bold**: left-aligned and a fifth larger, no quotation marks, the author on the right. **Framed**: Classic inside a thin frame within the cut line |
| Cards per page | 4, **6** or 8, in two columns (2, 3 or 4 rows) |
| Title | the shared **Title**, in the chosen [lettering font](#lettering), outlined with [Colour-in](#colour-in); empty prints "Words to keep" ("Palabras para guardar" in Spanish) |
| Paper | any; within 8 mm margins, below a 12 mm title band |

The cards print the picked quotes first, then your own. At most 200 quotes, each up to 600 characters with an author of up to 100; more is an error, shown in the preview. Each run of tabs, line breaks, other control characters or spaces in a quote or an author becomes one space, as for [habit names](#habit-tracker), and a quote left with no text is dropped.

How the cards are drawn:

| Part | Rule |
|------|------|
| Cards | fill the area in two columns, 4 mm apart both ways, above a band at the foot kept for the credit; every page is a full grid, and the cards left over on the last page are empty frames |
| Cut line | dashed grey (#8a919c) around each card; the Framed style adds a solid grey frame 2.5 mm inside it |
| Quote | in the chosen lettering font, wrapped to the card. It starts at 22, 18 or 15 pt for 4, 6 or 8 cards a page (Bold: 26.4, 21.6 or 18 pt) and shrinks by 0.5 pt until it fits; at 8 pt the last line that fits ends with "…" |
| Author | "— Name", in the Go font at 8 pt, 1.5 mm below the quote, cut with "…" to one line. The quote and author are centred down the card together |
| Credit | "Quotes from Wikiquote (CC BY-SA 4.0)" ("Citas de Wikiquote (CC BY-SA 4.0)" in Spanish), small, centred at the foot of each page that holds a Wikiquote quote; pages with only your own quotes have none |

The cards have their own cut lines, so the cut mode does not apply.

#### Quotes from Wikiquote

The search is the only part of **Make pages** that uses the network; typed quotes need none.

- **What is sent.** The name you type goes to `en.wikiquote.org` or `es.wikiquote.org` (the UI language), through the MediaWiki API, no key, with the app's contact User-Agent `bujo-helper/1.0 ( https://github.com/iribarren/bujo-helper )`, one request at a time. Only the name and the titles of the pages it reads are sent, nothing about your pages.
- **How the page is chosen.** A full-text search finds up to six pages; the best one is read, and the others are the **Other matches**. When the best page has no quotes the next ones are tried, at most three pages a search. A name with no page at all gives `no quotes found for "Name"`; pages without quotes show "No quotes found on Person." with the other matches still offered.
- **Which quotes.** The items of the page's top-level lists, without their nested source lines and reference marks. Sections that are not the person's own sourced quotes are skipped: Disputed, Misattributed, Attributed, Misquoted, Quotes about…, See also, Notes, References, External links and the like, and their Spanish equivalents (Atribuidas, Dudosas, Citas sobre…, Véase también, Referencias…). Quotation marks around a whole quote are removed. Quotes of 10 to 400 characters are kept, without duplicates, up to 60 a page.
- **Licence.** Wikiquote's text is licensed under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/) (the Spanish site's copyright policy still names 3.0), which asks for credit: the caption on each page with a Wikiquote quote is that attribution. You pick single quotes; the app never prints a page's whole list.
- **Copyrighted works.** Many quotes come from books, films and songs still under copyright and are on Wikiquote as short quotations under fair use. Printing a few for your own journal stays within that; don't publish sheets of them.

### Visited map

A vector map of the world or of one continent with the countries you have visited filled in, and optionally their names listed below it. The map data is embedded in the app: no network, no API key.

To make one: open **Make pages**, pick **Visited map**, choose the region, type the countries you have visited (for example `Spain, Japan, MX, Perú`), and click **Export PDF…**.

| Setting           | Rule |
|-------------------|------|
| Region            | **World** (default), Africa, Asia, Europe, North America, South America or Oceania |
| Visited countries | names or ISO codes, separated by commas, semicolons or new lines (see [matching](#how-countries-are-matched) below); blanks are ignored and duplicates count once |
| Colour            | the fill of visited countries: **teal** (#2a9d8f, default), coral (#ee8466), sage (#8fae7e) or lavender (#a393c9) |
| List countries    | on by default: prints "Countries visited: N" and the names under the map, in the UI language and the Go font |
| Title             | the shared **Title**, in the chosen [lettering font](#lettering); empty prints "Places I've been" ("Lugares que he visitado" in Spanish) |
| Font, Colour-in   | the shared [Lettering](#lettering) controls; Colour-in also changes the map, see below |
| Paper             | any; the page is portrait, within 8 mm margins, below a 12 mm title band |

The map fills the width at the top of the area. With the list, it shrinks (down to 60 % of the area's height) to leave room for the names, which take 2 to 4 columns at 9 pt, shrinking to 6 pt. If they still do not fit, long names end with "…" and the last cell reads "+K" for the K names left out. Without the list, or with no countries, the map is centred. The cut mode does not apply.

#### How countries are matched

- Each entry is an English or Spanish country name (`Germany`, `Alemania`, `United States`, `Estados Unidos`) or an ISO 3166-1 code, alpha-2 or alpha-3 (`DE`, `DEU`).
- Case, accents and punctuation are ignored (`peru`, `Perú` and `PERU` are the same); `&` reads as "and" and a leading "The" is dropped.
- Common other names work too: USA, UK, England and Scotland (the United Kingdom), Holland, Czech Republic, Ivory Coast, Burma, Türkiye, Korea (South Korea), DPRK, Holy See, DRC, Swaziland, Malvinas, and a few more.
- An entry that matches no country is an error, `unknown country "X"`; one that matches two is an error naming both. The only such name is the Spanish "San Martín" (Saint Martin and Sint Maarten): use the English name or the ISO code. The preview shows the error and export is blocked until it is fixed.

#### How the map is drawn

| Part | Rule |
|------|------|
| World | the Natural Earth projection (Šavrič et al.), from 58° S to 84° N: Antarctica is left out; a thin frame outlines the projection |
| Continents | Lambert azimuthal equal-area, centred on the continent; the view is a fixed longitude/latitude box per region, and neighbouring countries in view are drawn like the rest |
| Filled | unvisited land light grey (#e3e6ea), visited countries in the chosen colour, thin white borders |
| [Colour-in](#colour-in) | no fills: thin black outlines, the visited countries with a heavier outline, ready to colour by hand |
| Small countries | a visited country smaller than 1.5 mm both ways (Malta, Singapore, Vatican on the world map) also gets a dot at its centre, filled, or outlined with Colour-in |

#### Map data

- Natural Earth [1:50m Cultural Vectors, Admin 0 – Countries](https://www.naturalearthdata.com/downloads/50m-cultural-vectors/), release v5.1.2 (241 countries, Antarctica dropped). Natural Earth is in the public domain: its [terms of use](https://www.naturalearthdata.com/about/terms-of-use/) say the data may be used "in any manner, including modifying the content and design", that no permission is needed and that crediting the authors is unnecessary. Its multilingual names, including the Spanish ones, come from Wikidata (CC0).
- Borders are Natural Earth's default, de facto ones; disputed areas are drawn as Natural Earth draws them. Kosovo, Taiwan and Somaliland are their own entries.
- The embedded file `internal/sheet/mapdata/countries.bin` (about 150 KB) is the GeoJSON simplified (Douglas–Peucker, 0.02°), quantized to 0.001° and compressed. To regenerate it, run `go generate` in `internal/sheet`: it downloads the pinned release, checks its SHA-256, and writes the same bytes every time.
- Maps of a single country's regions (US states, provinces) are a planned follow-up.

### Recipe cards

Your recipes laid out as landscape cards to cut out, like index cards, or one recipe a page. Type them in, import them from a [Markdown file](#markdown-format), or both, with an optional photo of your own. With no recipes it prints blank cards to fill in by hand. Nothing is downloaded: see [no recipe source](#no-recipe-source).

To make one: open **Make pages**, pick **Recipe cards**, click **Add recipe** (or **Import…**), fill in the recipe, choose the card size, and click **Export PDF…**.

| Setting | Rule |
|---------|------|
| Recipes | the list of recipes, by title ("Untitled recipe" until it has one); click one to edit it, **×** removes it. **Add recipe** adds a blank one; **Import…** adds the recipes of a Markdown file after the ones you have |
| Title, Serves, Time | the recipe's title and its two meta fields, as you want them printed (`4`, `30 min`) |
| Ingredients, Steps | one per line; blank lines are dropped. Steps are numbered for you |
| Photo | **Choose…** picks a JPEG or PNG file of your own; the form shows its file name (the full path in the tooltip), and **Remove** takes it off. It is read when the page is drawn, so the file must stay where it is |
| Card size | **A6** (default), **4 × 6 in** or **Full page** |
| Font, Colour-in | the shared [Lettering](#lettering) controls, for the recipe titles. There is no shared **Title**: each card prints its recipe's |
| Paper | any; see below |

How the cards tile on each paper (8 mm margins, 4 mm apart, centred):

| Paper | A6 (148 × 105 mm) | 4 × 6 in (152.4 × 101.6 mm) | Full page |
|-------|-------------------|-----------------------------|-----------|
| **A4**    | 2 a page | 2 a page | 1 recipe a page |
| US Letter | 2 a page | 2 a page | 1 recipe a page |
| B5        | 2 a page | 2 a page | 1 recipe a page |
| A5        | 1 a page, scaled down to about 132 × 94 mm | 1 a page, scaled down to about 132 × 88 mm | 1 recipe a page |

A card that does not fit the paper within the margins (both card sizes on A5) is scaled down, its shape kept, to one a page; the layout runs in the smaller card and the text sizes stay the same, so more of it flows onto continuation cards.

How a card is drawn:

| Part | Rule |
|------|------|
| Cut line | dashed grey around each A6 or 4 × 6 card, as on [quote cards](#quote-cards); Full page has none. 4 mm inside it is the content |
| Title | centred on one line in the chosen [lettering font](#lettering), from 16 pt (20 pt on Full page) shrinking to 9 pt, then cut with "…"; outlined with [Colour-in](#colour-in) at 14 pt or larger |
| Meta line | "Serves 4 · Time 30 min" ("Raciones 4 · Tiempo 30 min" in Spanish), centred, 8 pt in the Go font; left out when both are empty |
| Columns | **Ingredients** (40 % of the width, bulleted) and **Steps** (60 %, numbered), 4 mm apart, each under its heading. The headings follow the UI language |
| Photo | centre-cropped to the width of the Ingredients column, at the top of it: at most 4:3 and 45 % of the column's height |
| Text | the Go font, from 9 pt (11 pt on Full page) shrinking by 0.5 pt until both columns fit the card, down to 6 pt |
| Continuation cards | at 6 pt, what does not fit flows line by line onto the next cards, titled "Pancakes (continued)" ("(continuación)" in Spanish), with no meta line or photo; an item may break across cards and the steps keep their numbers. A column heading prints only on cards where that column has lines |
| Blank cards | a writing line for the title, "Serves ___ Time ___", and both headings over writing lines. They fill the slots left on the last page, or a whole page when there are no recipes |

Limits: at most 50 recipes; a title of up to 100 characters; servings and time up to 40; up to 60 ingredients of 200 characters and 40 steps of 600; a photo path of up to 4096 bytes. More is an error naming the recipe by its number ("recipe cards recipe 3: 61 ingredients, want at most 60"), shown in the preview. Each run of tabs, line breaks, other control characters or spaces becomes one space, as for [habit names](#habit-tracker), and a recipe with no title, ingredients, steps or photo is dropped. A photo that is missing or not a readable JPEG or PNG is an error naming the recipe and the file name, also shown in the preview.

The cards have their own cut lines, so the cut mode does not apply.

#### Markdown format

**Import…** reads a Markdown file (`.md`, up to 1 MiB) of one or more recipes. A complete example:

```markdown
My recipes, exported from my notes app.

# Pancakes

Serves: 4
Time: 30 min
Photo: photos/pancakes.jpg

## Ingredients

- 200 g flour
- 1 tbsp sugar
- A pinch of salt
- 2 eggs
- 300 ml milk

### For the topping

- Maple syrup
- A handful of berries

## Steps

1. Whisk the flour, sugar and salt in a bowl.
2. Beat in the eggs, then the milk, until smooth.
3. Rest the batter for 10 minutes.
4. Cook ladlefuls in a hot buttered pan,
   1 to 2 minutes a side.

## Notes

Lovely with lemon too. This section is ignored.

# Lentil soup

**Serves:** 6
**Time:** 1 h

![Lentil soup](lentil-soup.png)

## Ingredients

* 300 g brown lentils
* 1 onion, chopped
* 2 carrots, diced
* 1.5 l vegetable stock

## Method

1. Soften the onion and carrots in a little oil.
2. Add the lentils and the stock and simmer for 40 minutes.
3. Season and blend half of it.
```

That is two recipes: Pancakes has 8 ingredients ("For the topping" is one of them) and 4 steps, the last one joined into one line; Lentil soup has 4 and 3.

- **Recipes.** A level-1 heading (`# Pancakes`) starts a recipe. Text before the first one is ignored.
- **Keys.** `Serves:` or `Servings:` and `Time:` lines set the meta line; in Spanish, `Raciones:` or `Porciones:` and `Tiempo:`. Keys are case-insensitive and may be bold or italic (`**Serves:** 6`, `**Serves**: 6`).
- **Photo.** A `Photo:` (`Foto:`) line, or a line with only an image (`![Lentil soup](lentil-soup.png)`). A relative path is relative to the Markdown file's folder, so keep the photos next to it; `~` is not expanded. Backticks or `<…>` around the path are removed.
- **Lists.** `## Ingredients` (`## Ingredientes`) and `## Steps`, `## Method`, `## Directions` or `## Instructions` (`## Pasos`, `## Preparación`, `## Elaboración`, `## Instrucciones`; accents optional) start a list. Items are `-`, `*` or `+` bullets or `1.` / `1)` numbers (your numbers are not kept: steps are numbered in order); a task box `[ ]` is removed; a plain line in a list is an item too. An indented line, or a line right after a bullet or number, continues the item.
- **Ingredient groups.** A deeper heading inside a list (`### For the topping`) becomes an item, so groups print as a line among the ingredients. A deeper heading that names a section (`### Steps`) starts that section.
- **Ignored.** Any other `##` heading and its content (Notes, Tips, a story about your grandmother), code blocks, horizontal rules, and text outside a list that is not a key or a photo. Bold, italic and backticks are removed and links become their text.

A file with no `#` recipe gives "no recipes found". A recipe over the [limits](#recipe-cards) is an error naming it by its number, and nothing is imported.

#### No recipe source

The cards download nothing: no recipe or food photo source is used. The only candidate with an API, [TheMealDB](https://www.themealdb.com/api.php), was checked on 2026-11-23 and left out:

- Its free test key "1" is for "development of your app or for educational use"; releasing an app publicly needs a paid supporter key ([API page](https://www.themealdb.com/api.php), [FAQ](https://www.themealdb.com/faq.php)).
- Its [terms of use](https://www.themealdb.com/terms_of_use.php) state no licence for the recipe images ("Most of our artwork is custom and is created by our users"), and say nothing about printing them.

Your own recipes and your own photos need no licence, so the page kind stays offline.

### Kitchen conversions

A ready-made one-page chart of kitchen conversions to glue in next to your recipes. It has no options; it is computed by the app, no network.

To make one: open **Make pages**, pick **Kitchen conversions**, choose the paper, and click **Export PDF…**.

| Table | Rows |
|-------|------|
| Volume | 1, 3/4, 2/3, 1/2, 1/3 and 1/4 cup, 1 tbsp, 1 tsp and 1 fl oz in ml |
| Weight | 1, 4 and 8 oz and 1 lb in g, and 1 kg in lb |
| Oven temperatures | °C, °F and gas mark 1 to 9, with the caption "Fan oven: 20 °C lower" |
| Cups to grams | one cup of flour, sugar, brown sugar, icing sugar, butter, rice, rolled oats, honey, cocoa powder, and milk or water, in g |

The figures follow the usual cookbook conventions:

- A US cup of 240 ml (so 1/3 cup is 80 ml); a tablespoon is 15 ml, a teaspoon 5 ml and a fluid ounce 30 ml, all rounded to the ml.
- Ounces and pounds in grams rounded to the gram (1 oz = 28.3495 g, 1 lb = 453.592 g); 1 kg = 2.2 lb, to 0.1 lb.
- The oven table is the standard cookbook table: gas mark n is 250 + 25 × n °F exactly, and its °C is rounded to a multiple of 10 (within 8 °C of the exact figure, so 170 °C sits next to 325 °F).
- Cups to grams are typical figures for each ingredient (flour 125 g, sugar 200 g, butter 227 g, which is 8 oz, honey 340 g, milk or water 240 g); weigh when it matters.

| Setting | Rule |
|---------|------|
| Title | the shared **Title**, in the chosen [lettering font](#lettering), outlined with [Colour-in](#colour-in); empty prints "Kitchen conversions" ("Conversiones de cocina" in Spanish) |
| Labels | the table headings, unit words (cup, tbsp, tsp; in Spanish taza, cda., cdta.), the gas mark column, the caption and the ingredient names follow the UI language. Unit symbols (ml, g, kg, oz, lb, fl oz, °C, °F) stay as they are |
| Numbers | fractions are written as 1/3, 3/4 (the Go font has no ⅓ or ⅔, and a mix of both forms reads unevenly); in Spanish, decimals take a comma (2,2 lb) |
| Paper | any; within 8 mm margins, below a 12 mm title band |

How the chart is drawn:

- Four tables two by two (volume and weight above, oven temperatures and cups to grams below), each half the width less a 6 mm gap. Every row is the same height, so the taller table of each pair fills its half of the height.
- Table headings in the lettering font, from 16 pt shrinking to 8 pt to fit the column. Cells in the Go font, all at one size, from 12 pt shrinking to 6 pt until every cell fits its row and column.
- Each table has a thin border and rules between its rows, no vertical ones. The oven and cups tables have a header row with a light grey fill; with [Colour-in](#colour-in) the fill is left off.

The cut mode does not apply.

## Lettering

Headers, titles and quote card text can print in one of ten bundled lettering fonts instead of the plain Go sans, and as hollow outlines to colour in by hand, the way bubble letters drawn with a pen are coloured. The controls sit in the **Lettering** fieldset of [Print sheet](#print-sheet) (for **Full page**, **Checklist** and **Review cards**) and [Make pages](#generated-pages) (every page kind).

1. Click **Font** (or focus it and press ↓). The button shows the chosen font in its own face; the list shows each font in its face, with its style group and a [badge](#colour-in). Arrow keys, Home and End move, Enter or Space picks, Escape closes.
2. Check **Colour-in (outlines only)** to print outlines. The hint under it says what the chosen font does.
3. Check the preview, then export.

Each tab keeps its own font and Colour-in until the app closes. With the **Default** font and Colour-in off, every PDF is the same as before lettering existed.

### What uses the font

| Text | Font |
|------|------|
| Full page and Checklist headers, Dot grid, Moon calendar, Month calendar, Habit tracker, Quote cards, Visited map and Kitchen conversions titles | the chosen font, outlined with Colour-in |
| Recipe card titles and Kitchen conversions table headings | the chosen font, outlined with Colour-in at 14 pt or larger; a title or heading shrunk below 14 pt to fit stays filled |
| Quote card text | the chosen font, outlined with Colour-in at 14 pt or larger; text shrunk below 14 pt to fit its card stays filled |
| Review card titles | the chosen font, always filled: at 9.5 pt they are too small for outlines |
| Captions, dates, stub text, spines, polaroid captions, checklist rows, credits, moon calendar labels, month calendar weekdays and day numbers, habit names and day numbers, quote authors and the Wikiquote credit, visited map country lists, recipe card meta lines, headings, ingredients and steps, conversion table cells and caption | always the Go font |

Headers still shrink from 20 pt down to 12 pt to fit, then end with "…". A bundled font fits into 96 % of the width, leaving room for swashes; the Default (Go) font keeps its bold header and the full width.

### Colour-in

Colour-in prints letters as black outlines (PDF text rendering mode 1) instead of fills. The outline is max(0.2, 0.25 × size / 20) mm wide, so 0.25 mm at 20 pt, and the next text prints filled again. What it does depends on the font's badge:

| Badge | Fonts | With Colour-in |
|-------|-------|----------------|
| **outline ok** | Default, Kaushan Script, Pacifico, Patrick Hand, Titan One, Bungee, Luckiest Guy | outlined at 14 pt or larger; smaller text stays filled |
| **hollow** | Bungee Outline, Londrina Outline | outlined by design, never stroked again; the box shows checked, and your own choice comes back with the next font |
| **no outline** | Sacramento, Amatic SC | too thin to outline: letters stay filled |

On a [moon calendar](#moon-calendar), Colour-in also draws the moons as outlines only (the disc and the terminator, no fills), ready to shade by hand, whatever the font. On a [visited map](#visited-map), it draws the countries as black outlines with no fills, the visited ones heavier. On [quote cards](#quote-cards), the quote text is outlined too when it prints at 14 pt or larger; the cut lines and frames do not change. On a [month calendar](#month-calendar) and a [habit tracker](#habit-tracker), only the title changes: their boxes are already outlines. On [recipe cards](#recipe-cards), the recipe titles are outlined at 14 pt or larger; photos, cut lines and writing lines do not change. On [kitchen conversions](#kitchen-conversions), the table headings are outlined too and the grey fill of the header rows is left off, so only outlines remain.

### Bundled fonts

| Font | Style group | Badge | Licence | Source |
|------|-------------|-------|---------|--------|
| Default (Go Regular, Go Bold) | Sans | outline ok | BSD 3-Clause | [`golang.org/x/image/font/gofont`](https://pkg.go.dev/golang.org/x/image/font/gofont) |
| Kaushan Script | Brush script | outline ok | SIL OFL 1.1 | `ofl/kaushanscript` |
| Pacifico | Brush script | outline ok | SIL OFL 1.1 | `ofl/pacifico` |
| Sacramento | Monoline script | no outline | SIL OFL 1.1 | `ofl/sacramento` |
| Patrick Hand | Handwritten | outline ok | SIL OFL 1.1 | `ofl/patrickhand` |
| Amatic SC (Bold) | Handwritten | no outline | SIL OFL 1.1 | `ofl/amaticsc` |
| Titan One | Bold display | outline ok | SIL OFL 1.1 | `ofl/titanone` |
| Bungee | Bold display | outline ok | SIL OFL 1.1 | `ofl/bungee` |
| Luckiest Guy | Bold display | outline ok | Apache 2.0 | `apache/luckiestguy` |
| Bungee Outline | Hollow | hollow | SIL OFL 1.1 | `ofl/bungeeoutline` |
| Londrina Outline | Hollow | hollow | SIL OFL 1.1 | `ofl/londrinaoutline` |

- The ten lettering fonts were downloaded on 2026-11-20 from [`github.com/google/fonts`](https://github.com/google/fonts) (branch `main`, the paths above) and ship unmodified, as static TrueType (`glyf`) files, in `internal/sheet/fonts/<id>/` next to their licence (`OFL.txt`, or `LICENSE.txt` for Luckiest Guy; `go/LICENSE` for the Go fonts). fpdf embeds only the glyphs each PDF uses.
- Every font covers the English and Spanish alphabets (á é í ó ú ñ ü ¿ ¡). Luckiest Guy and Londrina Outline lack Latin Extended letters (ő ł č), which matters only for future languages.
- **Settings → About** lists every font with its licence name; open an entry to read the licence text.
- There is no OpenType shaping or kerning (fpdf has none), so script fonts lose their contextual alternates.
- The picker and the preview load the fonts from the app's asset handler at `/fonts/<id>.ttf` (licence text at `/fonts/<id>/licence`), next to `/thumbs` and `/preview`, so the frontend bundle holds no copy.

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
go run ./cmd/covers -type products -o products "Nutella, Ferrero" "Earl Grey, Twinings" "3017620422003"
BGG_TOKEN=<token> go run ./cmd/covers -type boardgames -o boardgames "Catan, 1995" "Wingspan, 2019"
go run ./cmd/covers -type podcasts -o podcasts "The Daily, The New York Times" "Radiolab, WNYC Studios"
```

Flags must come before the items:

| Flag           | Default  | Meaning                                                    |
|----------------|----------|------------------------------------------------------------|
| `-type`        | `books`  | Media type: `books`, `movies`, `tv`, `albums`, `games`, `anime`, `manga`, `artworks`, `places`, `nature`, `products`, `boardgames` or `podcasts` |
| `-o`           | `covers` | Output folder                                              |
| `-country`     | `es`     | Store country code; its editions are preferred             |
| `-csv`         | (none)   | CSV file, added after the positional items                 |
| `-tmdb-key`    | (none)   | TMDB API key; overrides `TMDB_API_KEY` and the saved key   |
| `-igdb-id`     | (none)   | IGDB (Twitch) Client ID; overrides `IGDB_CLIENT_ID` and the saved one |
| `-igdb-secret` | (none)   | IGDB (Twitch) Client Secret; overrides `IGDB_CLIENT_SECRET` and the saved one |
| `-bhl-key`     | (none)   | Biodiversity Heritage Library API key, for nature; overrides `BHL_API_KEY` and the saved key |
| `-bgg-token`   | (none)   | BoardGameGeek token, for board games; overrides `BGG_TOKEN`, the saved token and the built-in one |

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

## Theme and sections shown

Both are in **Settings** and apply as soon as you click **Save**.

- **Theme**: **System** (the default) follows the system's light or dark setting; **Light** and **Dark** override it. Saved as `"theme"`: `""`, `"light"` or `"dark"`.
- **Sections shown**: a checkbox tree of the app's sections and the lists inside them:
  - **Download**: the media types.
  - **Print covers**: paper, formats, cover sizes, crop modes and cut modes. Paper and cut modes also apply to **Make pages**, which shares them.
  - **Make pages**: the page kinds.
- Untick **Download** or **Make pages** to hide its tab; the app then opens on the first shown tab. Their ticks are kept while they are hidden. **Print covers** is always shown.
- Each list has an all/none checkbox. Every list keeps at least one item: the last ticked one can't be unticked, and "none" leaves the list's default ticked.
- Unticked items disappear from their selector. When the remembered choice is hidden, the default is used, or the first shown item when the default is hidden too.
- Not affected: Print covers' **Cards for** selector always lists every media type.
- Saved as `"view"` in the settings file, listing what is hidden (`"hiddenSections"` and `"hidden"` by list), so media types, formats, papers and page kinds added in later versions show up ticked. Settings files from older versions show everything.

To add a section or a list to this tree, add it to `SECTIONS` in `frontend/src/view.ts`; a test fails when a tab has no entry there.

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

A build from source has no built-in [BoardGameGeek token](#boardgamegeek-token). To build one in, pass it to the linker; it is never committed:

```sh
# Linux (bash); drop -tags webkit2_41 on macOS
wails build -tags webkit2_41 -ldflags "-X bujo-helper/internal/buildinfo.BGGToken=$BGG_TOKEN"
go build -ldflags "-X bujo-helper/internal/buildinfo.BGGToken=$BGG_TOKEN" ./cmd/covers
```

On Windows (PowerShell), use `$env:BGG_TOKEN` instead of `$BGG_TOKEN` and leave out `-tags webkit2_41`.

The [release workflow](.github/workflows/release.yml) does this with the `BGG_TOKEN` repository secret (**Settings → Secrets and variables → Actions**). Without the secret, as in a fork, the release still builds, with no built-in token.

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
  src/                 main.ts (download tab and tabs), print.ts (print sheet tab), pages.ts (make pages tab), quotesForm.ts (the quote cards search and picks), recipesForm.ts (the recipe list and editor, import and photo), pageForms.ts (the option forms of the page kinds), preview.ts (live PDF preview), lettering.ts (font picker and Colour-in), settings.ts (Settings dialog and About), view.ts (the sections and lists the user can hide), viewConfig.ts (the Sections shown tree), theme.ts and style.css
  wailsjs/             generated bindings (committed)
cmd/covers/            command-line tool
internal/
  media/               domain types: Item, Candidate, Provider, MediaType, Registry
  catalog/             built-in media types and their providers
  providers/           books: Apple Books, Open Library, Amazon; movies: TMDB, IMDb, Wikidata → iTunes; TV: TMDB, IMDb; albums: MusicBrainz + Cover Art Archive, iTunes; podcasts: iTunes; games: IGDB, Steam; anime and manga: AniList; artworks: the Met, Cleveland; places: Wikidata → Wikimedia Commons; nature: Wikidata taxa → Wikimedia Commons; products: Open Food Facts; board games: BoardGameGeek
  match/               title/creator matching and ranking
  batch/               text lines and CSV parsing
  downloader/          concurrent batch downloads, local-edition preference, image validation, file naming, credit files
  locale/              country table: language and ISBN prefixes of local editions
  isbn/                ISBN-10/13 validation, conversion and extraction
  quotes/              Wikiquote search: a person's page and its quotes, sections filtered and text cleaned
  buildinfo/           values injected into release builds (the BoardGameGeek token)
  web/                 shared HTTP client, rate-limit throttle, headers and bounded reads
  settings/            user settings file (API keys, language, theme, shown sections, remembered options)
  gallery/             print sheet: folder listing and thumbnails for the UI
  preview/             in-memory store of preview PDFs served at /preview
  fonts/               serves the lettering fonts and their licences at /fonts/
  sheet/               print sheet: layouts (covers, review cards, spines, ticket stubs, full page, checklist, polaroid) and PDF export; generated page kinds (dot grid, moon calendar with Meeus moon phases, month calendar and habit tracker with shared calendar math, quote cards with fitted text, visited map with country matching and projections, recipe cards with continuation cards and the Markdown import, kitchen conversions); the lettering font registry and hollow text
    fonts/<id>/        the bundled lettering fonts (TTF) and their licence files, embedded
    mapdata/           the embedded Natural Earth country map (countries.bin) and its binary format
    mapgen/            the generator of countries.bin (`go generate` in internal/sheet)
build/                 Wails build assets (icons, manifests); binaries go to build/bin/
legacy/                the original Python script
```

## Adding a new media type

1. Implement `media.Provider` (`Name()` and `Search(ctx, item, opts)`) for each source, usually in `internal/providers`, plus `Label()` with the source's display name (for example `Open Library`), which the Download tab lists under the media types; a test checks every catalog provider has one. Return candidates best-first with their image URLs ordered best-first. Return an error wrapping `media.ErrNotFound` when there is no match. Set `Credit` on a candidate whose image needs an attribution file; the downloader saves it as `<image file>.credit.txt`.
2. Register a `media.MediaType` in `internal/catalog` with an ID, a label, a creator label (for example `Year` or `Artist`) and its providers in fallback order. Set `LocalEditions` only for types whose candidates carry an ISBN (books).

That's all. The UI lists media types from the registry and uses the creator label in its hints, so no frontend changes are needed.

## Legacy

`legacy/cover_downloader.py` is the original single-title Python script that this app replaces. It is kept for reference only and is not part of the build.

## License

[MIT](LICENSE)

### Third-party software

| Component | Licence | Use |
|-----------|---------|-----|
| [pdf.js](https://github.com/mozilla/pdf.js) (`pdfjs-dist`) | [Apache License 2.0](https://github.com/mozilla/pdf.js/blob/master/LICENSE) | draws the live PDF preview; bundled into the app by Vite with its worker, not loaded from a CDN |
| Lettering fonts: Kaushan Script, Pacifico, Sacramento, Patrick Hand, Amatic SC, Titan One, Bungee, Bungee Outline, Londrina Outline (9 fonts, from [Google Fonts](https://github.com/google/fonts)) | [SIL Open Font License 1.1](https://openfontlicense.org/open-font-license-official-text/) | [lettering](#lettering) of headers and titles; embedded unmodified, subset into each PDF |
| Luckiest Guy (from [Google Fonts](https://github.com/google/fonts)) | [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0) | [lettering](#lettering) of headers and titles; embedded unmodified, subset into each PDF |
| [Go fonts](https://go.dev/blog/go-fonts) (Go Regular, Go Bold) | BSD 3-Clause | every PDF's text and the Default lettering font |
| [Natural Earth](https://www.naturalearthdata.com/) 1:50m Admin 0 – Countries, v5.1.2 (names from Wikidata, CC0) | [public domain](https://www.naturalearthdata.com/about/terms-of-use/) | the [visited map](#map-data); simplified and embedded |
| "Powered by BGG" logo (`powered-by-bgg-rgb.svg`, `powered-by-bgg-reversed-rgb.svg`, from BGG's official logo set linked from the [XML API Terms of Use](https://boardgamegeek.com/wiki/page/XML_API_Terms_of_Use)) | BoardGameGeek's trademark, used as its terms require | credits BoardGameGeek in the Download tab and **Settings → About**, linked to BGG; embedded unmodified in `frontend/src/assets/` |

Each font's licence file ships in `internal/sheet/fonts/<id>/` and can be read in **Settings → About**, which also names pdf.js.
