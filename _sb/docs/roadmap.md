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
- Import pipeline: external EXIF plugins no longer stall it; the last walked group is
  emitted; changed files store fresh metadata. Viewer `/N` is reactive.
  `make build-plugins` works on a fresh clone.
- exiftool hardening: go-exiftool v0.5.1 (command timeout, no zombie processes,
  output kept on stderr errors); the extractor skips a group whose main file cannot
  be read instead of mixing in a sidecar's EXIF. Lazy image loading.
- Gallery on the original `Workers.puml` design: `wlayout` writes `layoutDb`, the page
  renders only the visible window via `liveQuery` (virtualisation), resize keeps the
  first visible photo in place (anchor). No `.clear()` on import; `dexie-observable`
  removed.
- Favicon, tab title.
- `perceplib` moved to `eggs-gd` (module `github.com/eggs-gd/perceplib`, v0.0.6) and
  vendored into Perceptrail as a git subtree instead of a submodule.
- Repo moved to `eggs-gd`, public; CI; git flow with rulesets; version derived from
  history with CI tags on `develop`.
- Safari: the logger no longer breaks the app (stack format).
- svebapp on current Svelte 5 / Kit practices: `$app/state`, no `svelte/store`
  (component state + `LiveQuery` on `createSubscriber`), `{@attach}`, `$derived`
  instead of state writes in effects, clsx-style `class`, no side effects in `load`.
- Runtime paths: build artifacts in `gontroller/.build/`, runtime data (config,
  database, caches) in `gontroller/.var/`; config paths are relative to the config
  file; `--config` flag instead of `.env`; HTTP address, CORS origins and exiftool
  path from config. Config sections are owned by their modules
  (`client.ServerConfig`, `model.DBConfig` with a driver switch: sqlite; postgres is
  a stub).

## Releases

- **0.2.0** — when the first perceptor works end to end (e.g. a primitive geo). Until
  then everything stays in `develop`.
- **First public release** — includes Docker.

## Next

- [ ] Thumbnails on the server: libvips via `bimg` (needs `brew install vips`), 400 px
      for tiles, 1600 px for the viewer, WebP; `/assets/:guid?size=…` falls back to the
      original. Fixes blank tiles (decoding originals) and HEIC in Chrome/Firefox.
- [ ] First perceptor end to end (primitive geo) → release 0.2.0.

## Core — product (gontroller)

- [ ] Own media-type table (`MediaKind`: image/raw/video/animated/sidecar) instead of
      system `mime` (a minimal Docker image loses `.mov/.heic/RAW`); an explicit role
      of each file in the group.
- [ ] Main file of a group: the photo in a Live Photo, deterministic for RAW+JPEG;
      Live Photo pairs by `ContentIdentifier`; one main item merged from all sidecars
      ([`Item flow.puml`](../puml/Item%20flow.puml)).
- [ ] `date`: the `FileModifyDate` fallback never parses (printed with a zone) — 1741
      of 2413 items have a zero date (files without EXIF: screenshots, messengers).
- [ ] `date`: time zones — in the core, not in the geo perceptor. Store the instant
      (UTC) + offset (minutes, NULL = unknown) + source tag; the offset must be a
      column: sqlite and Postgres `timestamptz` return UTC on read. Needs an explicit
      offset in the perceplib date API (+00:00 vs unknown). Offset, in order:
      1. `OffsetTimeOriginal` (videos: `Keys:CreationDate` with a zone);
      2. `DateTimeOriginal` − GPS UTC time, rounded to 15 min;
      3. GPS coordinates → IANA zone from an embedded dictionary
         (`github.com/ringsaturn/tzf`), offset via `time.LoadLocation` (DST, history).
         Not longitude/15: no DST, wrong at administrative borders;
      4. the server's time zone, marked as assumed.
      Tags by group (EXIF vs QuickTime: `QuickTime:CreateDate` is already UTC).
      Visible only once the gallery sorts/groups by date — do together.
- [ ] Embedded RAW preview (`PreviewImage`/`JpgFromRaw`) for the transcoder.

## Core — service (gontroller)

- [ ] Validator per [`Walker.puml`](../puml/Walker.puml): same / moved / duplicate /
      changed by `HashShort`; `finalizeWalk` for deleted files (`Deleted`), `Dirty`.
- [ ] Unreadable/broken files still become items (0×0): exiftool returns File tags
      even for garbage. Decide how to mark them (ignored? error state?).
- [ ] Fewer Info logs in `fswalker` (several per file).
- [ ] `TestLoadExternalPlugins` should load real `.so` files.

## Deployment (first release)

- [ ] Dockerfile: CGO (sqlite, libvips), exiftool from a `dist-*` release, fix `CMD`
      (`--config /data/config.yml`, `/data` as a volume = what `.var/` is in dev).

## Frontend

- [ ] Viewer: close without leaving the site on a direct `/N` open, Escape, arrow
      navigation.
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
