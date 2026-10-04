# Roadmap

Status as of 2026-10-04 (PR #25). What exists is described where it lives — the
module READMEs ([gontroller](../../gontroller/readme.md),
[importer](../../gontroller/internal/importer/README.md),
[library](../../gontroller/internal/library/README.md),
[perceptors](../../perceptors/readme.md), [svebapp](../../svebapp/README.md),
[perceplib chain](../../perceplib/chain/README.md)); why it is so and what was
rejected — [findings.md](findings.md); the target flows — [`../puml`](../puml).
This file holds only what is open and the designs not built yet.

## Done, by PR

One line each; the details are in the READMEs and the PRs.

- #13 dates and zones · #14 the import as small steps · #15 the cheap stage, the
  Apple Photos grouper, the asset contract · #16 every perceptor is a view of the
  sheet · #17 perceptor data kept by the core, geo · #18 views by URL, the info
  panel, the delta sync, debug / release, broken files · #19 the plugin loader test
  on real `.so`, RAW previews oriented · #20 several tabs, plain HTTP on a LAN ·
  #21 Apple Photos on demand (PhotoKit) · #22 providers listed · #23 providers as
  the chain's grouping (`internal/library`) · #24 the import in stages
  ([review](review-pr24.md)) · #25 the top level declarative, `internal/`,
  integration tests in `test/` · #26 import steps without `model`, `Missing` off
  the files' rows, perceptors' rows reconciled at start, the library contract
  narrowed by its consumers.

## Releases

- **0.2.0 — the first release**: the transcode (photos and video, hardware video
  encoding) and Docker. A release is an image someone installs; without the
  transcode HEIC does not show in Chrome, iPhone HEVC does not play, big originals
  slow the grid. Until then everything stays in `develop`.

## Next: before the transcodes

The rest of the PR #24 review ([review-pr24.md](review-pr24.md), its numbers), each
a PR of its own, so the next chains do not touch everything:

- [ ] The walk in batches: one read of the rows under the root, `CheckTime` stamped
      per page, rows created in batches (3.1); Photos' own files not written as
      rows, or measured and accepted (3.2).
- [ ] exiftool's pool returns an error instead of panicking (3.7);
      `internal/perceptor` under `-race` (3.8).

## Next: the expensive stage (transcode)

In steps, each its own PR (design below, "Expensive stage"):

1. [ ] **Photo renditions** — libvips on the CPU, the source chosen to avoid a full
   decode (Photos' JPEG, the HEIC's embedded thumbnail, a RAW's embedded JPEG), the
   DB-state queue, benchmarks on the real library.
2. [ ] **Video, software** — `libx264`, HDR → SDR, the hover clip, the poster; the
   codec → encoder table and the probe with a software fallback from day one.
3. [ ] **Docker** — the image (jellyfin-ffmpeg), a base compose with software
   encoding; the docs say a Mac with Photos runs the native binary.
4. [ ] **Hardware** — QSV (`hwaccel.qsv.yml`), VideoToolbox (native on a Mac),
   NVENC when there is one to test on; a macOS ImageIO HEIC decoder only if the
   benchmarks show HEIC is the bottleneck.

0.2.0 needs 1–3; 4 is wanted. Only the plain folder's items are rendered (a library
renders itself: Apple — on demand through PhotoKit).

## Next: providers — other libraries as sources

A library that keeps renditions is a **provider**: we read its assets and metadata
and ask it for a rendition when the client needs one; we render nothing it has.
The mechanism is done (`internal/library`, Apple Photos first).

- **The client knows only our server**: the provider is server-side, gontroller
  proxies the bytes — the key and the library's address never reach the browser.
  The client contract is the same for every provider (`asset.onDemand`,
  `asset.full`, `/items/:guid/rendition/...`).
- **No access to their databases** where there is an API (Immich, PhotoPrism change
  their schemas); a database or a catalog only where it is the library's own format
  (Apple, Lightroom, digiKam).
- **Their metadata as ours**: the provider's record goes into the group with
  exiftool's tag names, so the perceptors read it unchanged; their faces, people,
  albums, labels — perceptor data without an ML of our own.
- A cloud-only original is not described (name, format, weight) until a second
  provider: the abstraction comes from two, not one.
- An API provider has no files to walk: probably a chain of its own, `sync →
  identify → exif → commit`, sharing the stages after the gate.

| provider | how | what it gives | notes |
|---|---|---|---|
| **Apple Photos** | `Photos.sqlite` + files; PhotoKit on demand | renditions, edits, Live Photos, video renditions | done |
| **Immich** | REST API + an API key (`asset.read`, `asset.view`; `asset.download` for originals); Sync v2 | thumbnail / preview (~1440 px) / original, transcoded playback; faces, people, albums, CLIP search | **first** — the owner uses it daily. To check: API stability, Sync v2 from a non-mobile client, its video transcode policy |
| **PhotoPrism** | REST API + an app password | thumbnails `/api/v1/t/<hash>/<token>/<size>`, H.264 video; labels, faces, places | similar to Immich |
| **Lightroom Classic** | the catalog `.lrcat` (SQLite) + `.lrdata` previews | ratings, keywords, collections, edits; its previews | previews in Adobe's own format |
| **digiKam** | its SQLite / MySQL DB + the files | tags, faces, ratings | no server |
| **Nextcloud Memories** | WebDAV / Memories' API | previews, albums, faces (Recognize) | |
| **LibrePhotos** | REST API | faces, places | less alive |
| **Synology Photos** | DSM's API (unofficial) | many NAS users | the API is not stable |
| **Ente** | its export (`ente export`): decrypted files + metadata JSON | a folder with sidecars | no API for us |
| **Google Takeout** | folders + `*.supplemental-metadata.json` per file | a folder with sidecars | EXIF often stripped |
| Google Photos (live) | — | — | not possible: since 2025 its API sees only what the app created |

## Next: the asset from all its files

Libraries write sidecars, so an asset is a package (the original, RAW + JPEG, edits,
`.xmp` / `.aae`, the motion part); the grouping collects the whole package.

- [ ] **One item from the whole package**: rating, tags, captions, face regions from
      the XMP sidecars (merge reads every file's declared tags already). Which
      library writes what, and what wins when files disagree — per library.
- [ ] **Live Photo pairs by `ContentIdentifier`** in generic folders (today by name:
      a rename breaks the pair, two namesakes stick together).
- [ ] An `animated` kind (GIF, animated WebP / HEIC) shown moving.
- [ ] Storing the metadata package in the DB — when a consumer needs it (the info
      panel's raw exif, a perceptor re-run without the files).
- [ ] Reading the sidecars of keyed (Apple) groups (only the main file is read now).

## Next: Apple Photos — open

- [ ] Files Photos offloads (Optimize Mac Storage) must not hide or delete the item:
      the grouper gets them as gone now and may keep the asset.
- [ ] The Photos permission when gontroller is not started from a terminal
      (launchd): the binary would need its own (Info.plist, a stable signature).
- Supported schema: `ZASSET` (macOS 11+); older (`ZGENERICASSET`) — not planned.
- Later: albums, people (`ZPERSON` / `ZDETECTEDFACE`) → perceptors.

## The next chains — a starting idea

The contract (the import follows it): the top of a chain has only linear stages,
each named by what it yields, each a sub-chain in its own package with one `New`;
the top knows no tool. The rough stages, each to be worked out on its own:

- **Render** (the expensive pass): `feed → render → commit`. Feed: the DB-state
  queue of the plain folder's items lacking renditions. Render: a switch by kind
  (photo: libvips; video: ffmpeg; Live Photo), hardware inside. Commit: discard if
  the item was deleted or changed meanwhile, else the renditions and `Ready`.
- **Pixel perceptors (ML)**: `feed → pixels → ml → commit`. Pixels from the render
  chain or the provider (Photos would mean asking in bulk — local thumbnails may do
  for faces). Maybe a service of its own (goMLer).
- **Group perceptors** (journeys, face clusters, series, duplicates): `feed → group
  → commit`, over what changed.
- **Maintenance**: `find → prune` — renditions of deleted items, perceptor values,
  orphans.
- **API providers**: see Providers.

## Smaller open items

- [ ] Typed keys: GUIDs and providers' keys are plain `string`.
- [ ] Dockerfile (with 0.2.0): CGO (sqlite, libvips), jellyfin-ffmpeg, exiftool from
      a `dist-*` release, `CMD --config /data/config.yml`, `/data` a volume. A base
      compose (software encoding) + an override per accelerator
      (`hwaccel.qsv.yml`: `/dev/dri` and the `render` group; `hwaccel.nvenc.yml`).
- [ ] **Zoom in the viewer** — the wheel zoom worked badly and is gone. Look at
      Immich and Google Photos (they differ): around the pointer, pan, pinch,
      double-click, the original's pixels past the preview.
- [ ] Geo on a map (markers) — a perceptor UI slot.
- Maybe: the optimal (Dijkstra) layout for full relayouts (a switch, a resize),
  the greedy one appending. Code kept (`svebapp/src/lib/workers/layout/`); not
  planned.

## Later: the perceptor platform

A stable core first.

- Server → client events (New / Updated / Processed Item) per
  [`Client flow.puml`](../puml/Client%20flow.puml): a push would carry the same delta
  the client asks for now (`/items?since=`).
- ML as a separate service (goMLer) per [`ML Flow.puml`](../puml/ML%20Flow.puml).
- [ ] Geo sections by country / city names (an offline geocoder: Natural Earth /
      GeoNames) instead of the time zone's.
- [ ] **Colour — deterministic, no ML** (`ml_color` → `color`). Pixels: the cheap
      preview scaled to ~32×32 (~10–20 ms, once per photo). Per photo: a histogram
      in OKLab / CIELAB (~64 bins), 2–3 dominant colours (median cut —
      deterministic), mean lightness and chroma (greys apart). Views: **rainbow**
      (absolute, by the dominant hue, sections by the colours present) and/or
      **similar colours** (relative, the trail by histogram distance). To decide.
- [ ] **Relative perceptors (the trail)** with the first ML perceptor (faces): from
      the anchor to the nearest unseen photo, then the nearest to that, both ways;
      shared with the colour trail.
- UI slots (`item-panel`, `view`): a perceptor brings its own UI (map, faces).
- Reprocessing on a plugin version change; fsnotify instead of the periodic walk.
- Several codecs / formats at once (`codecs: [h264, av1]`, `formats: [avif, webp]`)
  — the asset contract is ready, but each is another encode: N× disk and time.

## Design: the expensive stage

The transcode belongs to the core (as in Immich, PhotoPrism, Jellyfin); ML apart.

**Two stages, two chains** (decided): show what we can as early as possible, never
what the browser cannot show.

```
import (as now):  walk -> … -> identify (cheap preview: what already exists) -> exif -> commit
                  Visible if something viewable, else Waiting
render:           feed (the next item needing work, from the DB)
                  -> transcode (photo | video | Live Photo) -> Ready
                  -> perceptors, full pass (on our previews): results replace the cheap ones
```

- **The queue is DB state, not a channel**: "items that still lack X" — `Visible` /
  `Waiting` / `Dirty` without outputs; a perceptor whose pass / version for the
  item is behind. New photos appear in it, deleted ones drop out, a move changes
  nothing (outputs live under the GUID: `cache/thumbs/<guid>/…`).
- At commit: deleted → discard (long work checks between stages); a different
  fingerprint → discard, still queued; only the path changed → keep.
- Several workers: an "in work since" mark; stale marks go back to the queue.
- Perceptors run on both passes (incremental refinement); per item and perceptor
  the pass done and the perceptor's version are stored.
- Transcoders take the whole asset (group), not a file; regenerated for `Dirty`,
  dropped for deleted. Motion previews: a short muted clip on hover, the poster
  otherwise.

**Photos — renditions**, libvips on the CPU; HEIC via libheif, RAW via its embedded
preview (or libraw). The win is not decoding faster but not decoding a 12 MP HEIC at
all:

- **the source, cheapest first**: a big-enough JPEG in the group (JPEG shrinks while
  it loads); the HEIC's embedded thumbnail for a tile; the full original last;
- **a decoder per type that can be swapped** (source → decode → transform →
  encode); libvips first;
- **benchmarks on the real library**: JPEG / HEIC → 400 / 1600, on the Mac and the
  i5 — images/s, CPU, peak RSS;
- **sizes: an array in the config**, long side px, default `[400, 1600]`; any array
  must work (800 for Retina tiles, 2560 for a 2K 32" — config experiments);
  previews, not copies: the original stays behind the viewer's Original;
- **format: one, in the config** — `webp` (default) or `avif` (~20–30 % smaller,
  10-bit / HDR / P3, 5–10× slower to encode; all current browsers).

**Video** — ffmpeg (**jellyfin-ffmpeg**: every hardware backend and HDR tone
mapping in one build).

- **Codec: one, in the config** (`h264` default; `hevc`, `av1`), mapped to the
  accelerator's encoder (`h264_qsv` / `_vaapi` / `_nvenc` / `_videotoolbox` /
  `libx264`).
- **Accelerator in the config**: `auto | none | qsv | vaapi | nvenc |
  videotoolbox`; a probe at start (a few frames of `testsrc`), software on failure.
- **The whole chain on the GPU** (`-hwaccel qsv -hwaccel_output_format qsv`,
  `scale_qsv`): no frame copies to the CPU.
- **HDR → SDR** — iPhone video is HLG / Dolby Vision HEVC: without tone mapping an
  H.264 copy is washed out (`vpp_qsv` / `tonemap_opencl` / `libplacebo`).
- Outputs: the viewer's video (sizes from the config), a muted hover clip, a poster.

| | encode | decode | in Docker |
|---|---|---|---|
| Intel QSV / VAAPI (dev box: i5-13500T, UHD 770) | H.264, HEVC 8/10 bit; AV1 only on Arc / Core Ultra | + AV1 | `/dev/dri`, `render` group |
| NVIDIA NVENC | H.264, HEVC; AV1 on Ada+ | + AV1 | NVIDIA Container Toolkit |
| AMD VAAPI | H.264, HEVC | | `/dev/dri` |
| Apple VideoToolbox | H.264, HEVC | | no: run the binary natively |

**Where it runs**: in gontroller by default — one compose, one process. The same
binary may later run in roles (`role: transcoder`) on a GPU box, taking work from
the same DB queue.

## Design: perceptor groups

Per-item values (`ProcessingMode` Single) are done (`api.NewStore[T]`, see the
[perceptors README](../../perceptors/readme.md)). **Groups** — later:

- Group perceptors produce groups of photos with data of their own: face clusters,
  album suggestions, journeys (time and place), series, duplicates.
- Typed the same way: `api.NewGroups[Trip]("journey", 1)`, `Trips.Put(Trip{…},
  guids)`; a table of groups and one of photo → group, kept by the core under the
  same rules (deletions, schema version), in the perceptor's file / schema.
- Not in the import chain: a group cannot be decided one photo at a time — a pass
  of its own after the import, from a DB-state queue.
- **Groups are sections**: the sheet by groups ("Lviv, May 2025"), photos in each by
  time; the side panel is the list of groups.
- Vectors (embeddings, colour histograms): blobs; nearest neighbours from an
  in-memory index (HNSW) built at load, later `sqlite-vec` / `pgvector`.

## Core vs. perceptors

The server owns what the gallery cannot display correctly or identify a photo
without, and what makes no sense to implement differently. Anything that adds its
own navigation axis or its own UI is a perceptor.

- **Core:** date (+ zone), size (+ orientation), media type, file identity and
  grouping, embedded preview, basic capture info (display only), user XMP / IPTC
  metadata (rating, tags, captions).
- **Perceptors:** geo and map, faces, objects, colour, camera / lens / exposure,
  series and bracketing, similar photos.
