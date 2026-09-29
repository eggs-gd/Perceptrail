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
- Validator per `Walker.puml` (V1–V5): walk safety (unreadable dirs skipped, no
  deletions after an incomplete/empty walk), deleted files (items soft-deleted,
  sidecars → `Dirty`), moves keep the GUID (also after a deletion), duplicates are new
  items, `Dirty`/not-`Ready` items are reprocessed, closer sets `Ready`. Tests on a
  temp library + sqlite.
- Dates (D1–D3): `FileModifyDate` parsed (with its zone; zero dates were most of the
  library), zero `0000:00:00` skipped, sub-seconds kept; the instant + offset
  (`DateOffset`, minutes) + `DateSource` (tag) + `DateZone` (how the offset was found).
  Zone chain: the tag's offset → local time − GPS UTC time (rounded to 15 min) →
  GPS coordinates → IANA zone (`tzf`, embedded, DST-aware) → the server's zone
  (assumed). Videos: zoned `CreationDate`, else QuickTime `CreateDate` = UTC.
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

One PR per feature (its steps are commits). The import chain C1–C7 is in its own
PR (`feature/import-chain`).

### Import chain: one small step per node

Graph: [`Import chain.puml`](../puml/Import%20chain.puml); gate and validator
details: [`Walker.puml`](../puml/Walker.puml). Before, `fswalker` did everything
(walk, MIME, grouping, files table, change detection, deletions) and the validator
hid inside the exif step, so groups and MIME were decided before EXIF was known.

- [x] **C1. fswalker = spam.** Every file found (path + stat), nothing else; walk
      safety from V1 stays.
- [x] **C2. Groups: a switch by source.** Groupers are plain decorators with their
      own buffer of open groups: a group goes out when it is complete, so every file
      closes at most one group. `generic`: sidecars by name, next to each other
      (name order), case-insensitive — one open group. Apple Photos: a stub branch;
      `photosLibraryEnabled = false` keeps the library in `generic` until the Photos
      milestone (there: the first file of the library loads the asset links from its
      DB, a group closes when all its files arrived; incomplete groups at the marker
      — e.g. cloud-only originals — to decide). The marker is broadcast to every
      branch and goes out with the grouper's last group.
- [x] **C3. Files gate.** The files table: new / changed (size, mtime) / never
      linked / item not Ready → pass, otherwise drop (no exiftool for unchanged
      files); `CheckTime`; deletions (V2) after the marker from every branch.
- [x] **C4. exif** for every file of the group (`-all`: the main file is not known
      yet), N steps in parallel on the same channels (it was one serial step with a
      pool of 5 processes).
- [x] **C5. mime.** Kind of every file: exif `MIMEType` → own extension table
      (`MediaKind`: image/raw/video/sidecar/other) → content sniff; rank: the main
      file is always the source — RAW > video > image; the JPEG of RAW+JPEG and the
      photo of a Live Photo are derivatives (sidecars); ties by size, name. No system
      `mime` tables (Docker).
- [x] **C6. validator** as its own step: links the group to the main file, a former
      main file that became a sidecar loses its item (a JPEG whose RAW appeared); same /
      changed / moved / duplicate; a moved item with complete outputs → `Ready`, no
      transcode (no outputs exist yet, so always).
- [x] **C7. transcode switch** by kind (photo / video / Live Photo; stubs pass the
      item on), then plugins, closer → `Ready`. Finished items are drained (the
      closer used to block after 1000 items: nothing read the channel).
- [ ] Later: derivatives in a group (the JPEG of a RAW, the photo of a Live Photo)
      as ready previews — saves a transcode; Live Photo pairs checked by
      `ContentIdentifier` (today by name); `animated` kind.

### Dates and time zones

D1–D3 done (PR #13), see Done.

- [ ] **D4. API and client — to discuss.** Maybe not needed: if server and client
      normalise dates the same way, the API needs no separate zone. Sorting/grouping
      the gallery by date is separate (it makes all of this visible).

### Photos library (`*.photoslibrary`) as its own source — separate milestone

The bundle is not a folder of photos: 30 311 files, of them 18 154 `database/search`,
8 989 `resources/caches`, 1 780 `originals/<0-F>/<UUID>.<ext>` (no original names),
1 143 `resources/derivatives`. Yet 1 844 of 2 413 gallery items come from it. The
truth is in `database/Photos.sqlite`: original filename, date + time zone
(`ZADDITIONALASSETATTRIBUTES.ZTIMEZONEOFFSET/ZTIMEZONENAME`), GPS, Live Photo pairs,
edits, trashed/hidden, favourites, albums. Name-based grouping does not apply.
Private Apple format: the schema changes between macOS versions (osxphotos is the
reference); reading needs Full Disk Access for the process (TCC).

- [ ] **P0. Spike.** Read-only `Photos.sqlite` (`mode=ro`, the library may be open in
      Photos) on this macOS: map asset UUID → files (original, Live Photo video,
      edited render, `.aae`), date/zone, GPS, kind, trashed/hidden, cloud-only
      originals (Optimize Mac Storage: no local original). Findings entry; decide the
      supported macOS range.
- [ ] **P1. Source step.** The walker meets a `*.photoslibrary` directory →
      `SkipDir` and hands it to a library reader that emits the same groups
      (`[]FileDto`) into the chain: one group per asset — main original + Live Photo
      video / edited render as sidecars; the GUID from the asset UUID (stable).
      Everything else in the bundle is not scanned. Trashed assets are not emitted
      (→ `Deleted` via V2). Cloud-only: skip (or a derivative as a fallback — decide
      in P0).
- [ ] **P2. Library metadata.** The asset's DB attributes join the group as a
      virtual metadata record (e.g. `Photos:*` tags next to exiftool's), so the core
      plugins use them: the zone from the library comes first in D3, the original
      filename, favourite/hidden.
- [ ] **P3. Tests** on a fixture library (minimal `Photos.sqlite` + files).
- Later: albums, people (`ZPERSON`/faces) → perceptors.

### Then

- [ ] Thumbnails on the server: libvips via `bimg` (needs `brew install vips`), 400 px
      for tiles, 1600 px for the viewer, WebP; `/assets/:guid?size=…` falls back to the
      original; regenerated for `Dirty`, dropped for `Deleted`. Fixes blank tiles
      (decoding originals) and HEIC in Chrome/Firefox.
- [ ] First perceptor end to end (primitive geo: map, markers) → release 0.2.0.

## Core — product (gontroller)

- Own media-type table and the main file of a group → **C5** (import chain).
- [ ] One main item merged from all sidecars
      ([`Item flow.puml`](../puml/Item%20flow.puml)).
- [ ] Embedded RAW preview (`PreviewImage`/`JpgFromRaw`) for the transcoder.

## Core — service (gontroller)

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
