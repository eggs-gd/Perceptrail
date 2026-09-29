# Roadmap

Status as of 2026-09-29. Details and reasons — [findings.md](findings.md).
Target architecture — the diagrams in [`../puml`](../puml).

## Done

- exiftool moved to `github.com/eggs-gd/go-exiftool` (v0.5.0); a tested ExifTool
  13.55 distribution is published as the `dist-13.55` release.
- Go 1.27.1 and fresh dependencies in `perceplib`, `gontroller`, `perceptors/*`;
  `perceplib` v0.0.5 (logger decorator, no exiftool).
- Gallery: stable resize and streaming (in-memory layout in the worker,
  `replace`/`upsert`, absolute tiles with transitions, a wave for tiles that change
  rows, applied once per frame).
- svebapp: Vite 8, Svelte 5.57, Kit 2.70, TS 6; `svelte-check` — 0 errors.
- Favicon, tab title.
- `perceplib` moved to `eggs-gd` (module `github.com/eggs-gd/perceplib`, v0.0.6) and
  vendored into Perceptrail as a git subtree instead of a submodule.

## Next

- [ ] Commit the current state of `feature/perceptors`.
- [ ] Transfer the `Perceptrail` repo to `eggs-gd`, update local remotes.
- [ ] Experiment: `liveQuery` between worker and UI on Dexie 4 (do events arrive,
      latency, without `.clear()` on import). If yes — back to the original design
      of [`Workers.puml`](../puml/Workers.puml): UI subscribes to the visible window
      of `LayoutDB` + virtualisation.
- [ ] `Img.svelte`: `loading="lazy"` (the browser currently fetches every image).
- [ ] Verify and fix the possible pipeline stall with external EXIF plugins
      (`ExifPluginProcessor`, `ExifPerceptor` vs `ExifCorePerceptor`).
- [ ] `exifextractor`: shadowed `err` → an empty `RawExif` is added to the group.
- [ ] go-exiftool: per-command timeout with restart, `Wait` after `Kill`, handle the
      `start()` error in `restart`.

## Core correctness (gontroller)

- [ ] `fswalker`: own media-type table (`MediaKind`: image/raw/video/animated/sidecar)
      instead of system `mime`; an explicit role of each file in the group; the main
      file of a Live Photo is the photo; deterministic main file for RAW+JPEG.
- [ ] Validator per [`Walker.puml`](../puml/Walker.puml): same / moved / duplicate /
      changed by `HashShort`; `finalizeWalk` for deleted files (`Deleted`), `Dirty`.
- [ ] `date`: time zones (`OffsetTimeOriginal`, GPS UTC), parse dates with a zone.
- [ ] Identity: Live Photo pairs by `ContentIdentifier`; one main item merged from all
      sidecars ([`Item flow.puml`](../puml/Item%20flow.puml)).
- [ ] Transcoder (photo/video processors from Item flow): thumbnails, embedded RAW
      preview (`PreviewImage`/`JpgFromRaw`).
- [ ] HTTP port and exiftool path from config (`:1323` is hardcoded; the `database`
      section of the config is unused — SQLite).
- [ ] Dockerfile: CGO (sqlite, libvips), exiftool from a `dist-*` release, fix `CMD`.
- [ ] Fewer Info logs in `fswalker` (several per file).
- [ ] `TestLoadExternalPlugins` should load real `.so` files.

## Frontend

- [ ] `$app/stores` → `$app/state`.
- [ ] Drop `.clear()` on import of `itemsDb`/`layoutDb` (race between contexts).
- [ ] Decide on `dexie-observable` (legacy add-on).
- [ ] Virtualisation (render visible rows only).
- [ ] Viewer: update on `/3 → /4`, direct open of `/N`, close without leaving the
      site, Escape, arrow navigation.
- [ ] Optimal (Dijkstra) layout for an already loaded gallery — optional.

## Later: the perceptor platform

Don't rush — a stable core first.

- Server → client events (New / Updated / Processed Item) as designed in
  [`Client flow.puml`](../puml/Client%20flow.puml) (MQTT): the client draws a
  preloader, fixes the grid, then the thumbnail. Until then — incremental
  `/items?since=` + tombstones as a fallback.
- ML as a separate service (goMLer) per [`ML Flow.puml`](../puml/ML%20Flow.puml):
  consumes Processed Item, runs ML plugins, returns metadata.
- Storage for perceptor data (e.g. `item_attrs(item_id, perceptor, key, value)`), so
  a plugin writes its own data without changing `ItemDto`.
- Perceptor registry for the client (`GET /perceptors`: axes, filters, UI bundle) and
  UI slots (`item-panel`, `view`, `filter`) — a perceptor can bring its own UI (map,
  face management).
- Perceptor routes (`/p/<name>/…`), a common query/filter API (set intersection).
- `ItemGroup` processing mode (series, Live Photo, clusters, duplicates).
- Reprocessing on plugin version change; fsnotify instead of a one-shot walk.

### Core vs. perceptors

Rule: the server owns what the gallery cannot display correctly or cannot identify a
photo without, and what makes no sense to implement differently. Anything that adds
its own navigation axis or its own UI is a perceptor.

- **Core (exif_core):** date (+ time zone), size (+ orientation), media type, file
  identity and grouping, embedded preview, basic capture info (display only), user
  XMP/IPTC metadata (rating, tags, captions).
- **Perceptors:** geo and map, faces (`RegionInfo` as ready-made labels), objects,
  colour, navigation by camera/lens/exposure, series and bracketing, "similar photos".
