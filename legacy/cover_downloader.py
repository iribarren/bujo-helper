#!/usr/bin/env python3
"""Download a book's cover image given its title.

Sources are tried in order until one succeeds:
  apple        Apple Books (iTunes Search API) - high resolution, no key
  openlibrary  Open Library covers API         - free/open, smaller images
  amazon       Amazon search scraping          - may hit bot checks

Usage:
    python cover_downloader.py "The Name of the Wind"
    python cover_downloader.py "Cien años de soledad" -c es -o covers/
    python cover_downloader.py "Dune" --sources amazon,apple
"""

from __future__ import annotations

import argparse
import re
import sys
import time
import unicodedata
from pathlib import Path
from typing import Callable
from urllib.parse import quote_plus

import requests
from bs4 import BeautifulSoup

TIMEOUT = 15

API_HEADERS = {"User-Agent": "cover-downloader/1.0 (python-requests)"}
BROWSER_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/126.0 Safari/537.36"
    ),
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.9",
}

# Country code -> Amazon domain, for the few that are not just the code itself.
AMAZON_DOMAINS = {"us": "com", "gb": "co.uk", "uk": "co.uk", "jp": "co.jp", "br": "com.br", "mx": "com.mx", "au": "com.au"}


class CoverNotFound(Exception):
    pass


# A source returns (display_name, candidate_image_urls ordered best-first).
SourceResult = tuple[str, list[str]]


# ---------------------------------------------------------------- matching --

def normalize(text: str) -> str:
    text = unicodedata.normalize("NFKD", text).encode("ascii", "ignore").decode()
    return re.sub(r"[^a-z0-9]+", " ", text.lower()).strip()


def rank_by_title(items: list, title: str, get_title: Callable) -> list:
    """Stable-sort search hits so the closest title matches come first."""
    query = normalize(title)

    def score(item) -> int:
        full = normalize(get_title(item) or "")
        main = normalize(re.split(r"[:(]", get_title(item) or "")[0])
        if main == query or full == query:
            return 0
        if full.startswith(query) or query.startswith(main):
            return 1
        if query in full:
            return 2
        return 3

    return sorted(items, key=score)


def pick(items: list, index: int) -> object:
    if len(items) <= index:
        raise CoverNotFound(f"no matching book found ({len(items)} results)")
    return items[index]


# ------------------------------------------------------------------ apple --

def from_apple(session: requests.Session, title: str, country: str, index: int) -> SourceResult:
    resp = session.get(
        "https://itunes.apple.com/search",
        params={"term": title, "media": "ebook", "limit": 25, "country": country},
        headers=API_HEADERS,
        timeout=TIMEOUT,
    )
    resp.raise_for_status()
    hits = [h for h in resp.json().get("results", []) if h.get("artworkUrl100")]
    hit = pick(rank_by_title(hits, title, lambda h: h.get("trackName")), index)
    # Asking for an oversized box returns the original artwork resolution.
    url = hit["artworkUrl100"].replace("100x100bb", "10000x10000bb")
    return f"{hit.get('trackName')} - {hit.get('artistName')}", [url]


# ------------------------------------------------------------ openlibrary --

def from_openlibrary(session: requests.Session, title: str, country: str, index: int) -> SourceResult:
    resp = session.get(
        "https://openlibrary.org/search.json",
        params={"title": title, "limit": 25, "fields": "title,author_name,cover_i"},
        headers=API_HEADERS,
        timeout=TIMEOUT,
    )
    resp.raise_for_status()
    docs = [d for d in resp.json().get("docs", []) if d.get("cover_i")]
    doc = pick(rank_by_title(docs, title, lambda d: d.get("title")), index)
    author = ", ".join(doc.get("author_name", [])[:2])
    # default=false makes missing covers return 404 instead of a blank image.
    url = f"https://covers.openlibrary.org/b/id/{doc['cover_i']}-L.jpg?default=false"
    return f"{doc.get('title')} - {author}", [url]


# ----------------------------------------------------------------- amazon --

# Amazon image URLs embed size modifiers, e.g. "81abc._AC_UY218_.jpg".
# Removing the "._..._" segment returns the original full-size image.
SIZE_MODIFIER = re.compile(r"\._[^/]*_(?=\.\w+$)")


def is_amazon_search_page(html: str) -> bool:
    # A real search page (even with zero results) has the results container.
    # Bot checks (captcha, Akamai "bm-verify" interstitial) don't.
    return "s-main-slot" in html and "bm-verify" not in html


def amazon_asin_image(asin: str) -> str:
    # "_SCRMZZZZZZ_" asks Amazon's image service for the large (~1500px) version.
    return f"https://images-na.ssl-images-amazon.com/images/P/{asin}.01._SCRMZZZZZZ_.jpg"


def from_amazon(session: requests.Session, title: str, country: str, index: int, retries: int = 2) -> SourceResult:
    domain = AMAZON_DOMAINS.get(country, country)
    url = f"https://www.amazon.{domain}/s?k={quote_plus(title)}&i=stripbooks"
    for attempt in range(1, retries + 1):
        resp = session.get(url, headers=BROWSER_HEADERS, timeout=TIMEOUT)
        if resp.ok and is_amazon_search_page(resp.text):
            break
        if attempt < retries:
            time.sleep(3 * attempt)
    else:
        raise CoverNotFound(f"blocked by bot check (HTTP {resp.status_code})")

    soup = BeautifulSoup(resp.text, "html.parser")
    results = [
        r
        for r in soup.select('div[data-component-type="s-search-result"]')
        if r.get("data-asin") and r.select_one("img.s-image")
    ]
    result = pick(results, index)
    img = result.select_one("img.s-image")
    name_tag = result.select_one("h2")
    name = name_tag.get_text(strip=True) if name_tag else img.get("alt", "cover")
    search_image = SIZE_MODIFIER.sub("", img["src"])

    formats = []
    for link in result.select("a.a-size-base.a-link-normal.s-underline-text[href*='/dp/']"):
        match = re.search(r"/dp/([A-Z0-9]{10})", link["href"])
        if match:
            formats.append((link.get_text(strip=True).lower(), match.group(1)))

    # Audiobook-first results show square artwork; prefer the print/Kindle cover.
    is_audio = lambda fmt: "audio" in fmt or "audible" in fmt  # noqa: E731
    if formats and is_audio(formats[0][0]):
        print_asins = [asin for fmt, asin in formats if not is_audio(fmt)]
        return name, [amazon_asin_image(a) for a in print_asins] + [search_image]
    return name, [search_image]


SOURCES: dict[str, Callable[..., SourceResult]] = {
    "apple": from_apple,
    "openlibrary": from_openlibrary,
    "amazon": from_amazon,
}


# --------------------------------------------------------------- download --

def fetch_image(session: requests.Session, urls: list[str]) -> tuple[str, bytes]:
    for url in urls:
        resp = session.get(url, headers=BROWSER_HEADERS, timeout=TIMEOUT)
        # Missing images come back as 404s or tiny placeholder GIFs.
        if resp.ok and resp.headers.get("Content-Type", "").startswith("image/") and len(resp.content) > 1024:
            return url, resp.content
    raise CoverNotFound("found the book but could not download its cover image")


def safe_filename(text: str, max_len: int = 100) -> str:
    cleaned = re.sub(r"[^\w\s-]", "", text).strip()
    return re.sub(r"\s+", "_", cleaned)[:max_len] or "cover"


def resolve_output(output: Path, title: str, image_url: str) -> Path:
    ext = Path(image_url.split("?")[0]).suffix or ".jpg"
    if output.is_dir() or not output.suffix:
        output.mkdir(parents=True, exist_ok=True)
        return output / f"{safe_filename(title)}{ext}"
    output.parent.mkdir(parents=True, exist_ok=True)
    return output


def download_cover(title: str, output: Path, sources: list[str], country: str = "us", index: int = 0) -> Path:
    errors = []
    with requests.Session() as session:
        for source in sources:
            try:
                name, candidates = SOURCES[source](session, title, country, index)
                image_url, data = fetch_image(session, candidates)
            except (CoverNotFound, requests.RequestException, ValueError) as exc:
                print(f"[{source}] {exc}", file=sys.stderr)
                errors.append(source)
                continue

            path = resolve_output(output, title, image_url)
            path.write_bytes(data)
            print(f"Source: {source}")
            print(f"Found:  {name}")
            print(f"Image:  {image_url}")
            return path

    raise CoverNotFound(f"no cover found in any source ({', '.join(errors)})")


def parse_sources(value: str) -> list[str]:
    sources = [s.strip().lower() for s in value.split(",") if s.strip()]
    unknown = [s for s in sources if s not in SOURCES]
    if unknown or not sources:
        raise argparse.ArgumentTypeError(f"unknown source(s): {', '.join(unknown)}; choose from {', '.join(SOURCES)}")
    return sources


def main() -> int:
    parser = argparse.ArgumentParser(description="Download a book cover by title.")
    parser.add_argument("title", help="Book title (add the author for better matches)")
    parser.add_argument("-o", "--output", default=".", help="Output file or directory (default: .)")
    parser.add_argument(
        "-s", "--sources", type=parse_sources, default=list(SOURCES),
        help=f"Comma-separated sources to try in order (default: {','.join(SOURCES)})",
    )
    parser.add_argument(
        "-c", "--country", default="us",
        help="Store country: us, es, gb, de, fr... Used by Apple and Amazon (default: us)",
    )
    parser.add_argument("-i", "--index", type=int, default=0, help="Use the N-th best match (default: 0)")
    args = parser.parse_args()

    try:
        path = download_cover(args.title, Path(args.output), args.sources, args.country.lower(), args.index)
    except CoverNotFound as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1

    print(f"Saved:  {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
