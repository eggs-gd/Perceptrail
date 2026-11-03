# The demo

What the landing and the README show of the gallery: a recording of the perceptors at
work, over a **demo library** — public photos, never a real archive (the
organization's rule for every screenshot).

```bash
landing/demo/make.sh <work dir>     # or: npm run demo -- <work dir>, in landing/
```

It makes the library from the manifest, runs a test pair over it (gontroller on :1329
with the geo plugin, the gallery on :5174 — never the owner's :1323 / :5173), waits for
the renditions, records, and writes:

| file | where it is shown |
|---|---|
| `landing/static/demo/perceptors.webm`, `.mp4` | the landing, the perceptors' block |
| `landing/static/demo/poster.webp` | the landing's hero, the video's poster |
| `docs/images/perceptors.gif` | the README |

Needs exiftool, libvips, ffmpeg, Google Chrome (driven by `playwright-core`; no browser
or ffmpeg of Playwright's own is downloaded).

## The library

`manifest.json` — photos from Wikimedia Commons, **public domain or CC0 only** (nothing
to attribute in a frame), geotagged, around eleven cities in different time zones;
every shape from panoramas to portraits (the justified grid shows them), at most three
of one author in a city. Each entry keeps its Commons page, license and author.
`fetch.mjs` downloads a 2000 px rendition of each and writes its date and place into
its EXIF: the Commons date when there is one, else the first dated day of its city —
one trip a city; the Commons coordinates. `collect.mjs` picks a new manifest (only
when the library should change).

## The frames

About 14 s, 1280 × 800, a loop:

1. The gallery in **Size**, scrolled into the sheet (at its very top a switch keeps the
   top, not a photo).
2. A cursor comes to the photo in the middle; a ring marks it.
3. The cursor goes to **Date**; its hint shows — the perceptor's own help from
   `/perceptors`. A click: the sheet is ordered by date; the marked photo stays where
   it was, the ring with it.
4. The same with **Place**: ordered by where the photos were taken.

The cursor, the ring and the hint are drawn over the page by `record.mjs` (a headless
recording has no cursor or tooltips); the gallery itself is unchanged.
