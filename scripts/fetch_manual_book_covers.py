#!/usr/bin/env python3

from __future__ import annotations

import os
import re
import sys
import urllib.request
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUTPUT_DIR = ROOT / "static" / "book-covers"
USER_AGENT = "athulsuresh-blog/1.0"


MANUAL_SOURCES = {
    "book-review-all-systems-red": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL26818660M-L.jpg?default=false",
    },
    "book-review-artificial-condition": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL26960367M-L.jpg?default=false",
    },
    "book-review-can-t-hurt-me-master-your-mind-and-defy-the-odds": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL28024418M-L.jpg?default=false",
    },
    "book-review-house-of-huawei-the-secret-history-of-china-s-most-powerful-company": {
        "type": "page-og-image",
        "url": "https://www.penguinrandomhouse.com/books/721789/house-of-huawei-by-eva-dou/",
    },
    "book-review-kafka-on-the-shore": {
        "type": "page-og-image",
        "url": "https://www.penguinrandomhouse.com/books/118718/kafka-on-the-shore-by-haruki-murakami/",
    },
    "book-review-shogun": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL7518154M-L.jpg?default=false",
    },
    "book-review-sum-tales-from-the-afterlives": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL16873074M-L.jpg?default=false",
    },
    "book-review-the-phoenix-project-a-novel-about-it-devops-and-helping-your-business-win": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL25430556M-L.jpg?default=false",
    },
    "book-review-you-deserve-each-other": {
        "type": "direct",
        "url": "https://covers.openlibrary.org/b/olid/OL27947259M-L.jpg?default=false",
    },
}


def main() -> int:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    downloaded = 0
    skipped = 0

    for slug, source in MANUAL_SOURCES.items():
        target = OUTPUT_DIR / f"{slug}.jpg"
        if target.exists():
            skipped += 1
            continue

        try:
            image_url = source["url"]
            if source["type"] == "page-og-image":
                image_url = extract_og_image(source["url"])
                if not image_url:
                    print(f"no og:image found for {slug}", file=sys.stderr)
                    continue

            download_file(image_url, target)
            downloaded += 1
            print(f"downloaded {slug} <- {image_url}")
        except Exception as exc:
            print(f"failed {slug}: {exc}", file=sys.stderr)
            continue

    print(f"downloaded={downloaded} skipped={skipped}")
    return 0


def extract_og_image(page_url: str) -> str | None:
    request = urllib.request.Request(page_url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(request, timeout=30) as response:
        html = response.read().decode("utf-8", errors="ignore")

    patterns = [
        r'<meta[^>]+property=["\']og:image["\'][^>]+content=["\']([^"\']+)["\']',
        r'<meta[^>]+content=["\']([^"\']+)["\'][^>]+property=["\']og:image["\']',
        r'<meta[^>]+name=["\']twitter:image["\'][^>]+content=["\']([^"\']+)["\']',
    ]
    for pattern in patterns:
        match = re.search(pattern, html, re.IGNORECASE)
        if match:
            return match.group(1)
    return None


def download_file(url: str, target: Path) -> None:
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(request, timeout=30) as response:
        content_type = response.headers.get("Content-Type", "")
        if not content_type.startswith("image/"):
            raise RuntimeError(f"unexpected content type {content_type!r} for {url}")
        data = response.read()
    target.write_bytes(data)


if __name__ == "__main__":
    raise SystemExit(main())
