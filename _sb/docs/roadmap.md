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
      `groups.appleEnabled = false` keeps the library in `generic` until the Photos
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
- [x] **Repeated walks.** `rescan` (default 1 min) after the last group of the
      previous walk is processed, not after the walk: walks never overlap.
      Known cost: a group that fails (e.g. exiftool returns nothing) is retried on
      every walk — see "broken files".
- [ ] Later: derivatives in a group (the JPEG of a RAW, the photo of a Live Photo)
      as ready previews — saves a transcode; Live Photo pairs checked by
      `ContentIdentifier` (today by name); `animated` kind.

### Cheap stage: show what exists (with the Apple Photos grouper)

One PR (`feature/cheap-stage`). Photo and video transcode are the next milestones.

- [x] **S1. Visible / Waiting and the cheap preview** for generic groups: the main
      file if the browser shows it (JPEG, PNG, WebP, AVIF…; H.264 video), else the
      biggest viewable derivative, else an embedded preview (`exiftool -b` into
      `cache/previews/<guid>/`). HEIC (no extractable preview in HEIF) and HEVC wait.
      `/items`: Visible + Ready, `previewMime`; `/assets/:guid`: the preview.
- [x] **S2. Minimal Apple grouper**: asset links from a copy of the DB, one group
      per asset (trashed skipped), files ordered by what to show first (render →
      ~2000 px → ~1000 px → master → `.THM`); a group key = the asset UUID as the
      item's GUID (a changing main file — derivative, then the downloaded original —
      keeps the item); `appleEnabled = true`.
- [x] **S3. Minimal DB metadata** as a virtual exif record with exiftool's tag names
      (`DateTimeOriginal`, `OffsetTimeOriginal`, `ImageWidth`, GPS…): date + zone,
      dimensions, GPS — the plugins work unchanged; required for cloud-only assets.
- [x] **S4. Tests**: a fixture library (grouper, the whole chain, DB metadata).
- [x] **S5. Roles and sizes of the asset's files** (decided: the client gets the
      whole asset once and decides what to show when). Role per file: `original`
      (the source), `edit`, `still` (a viewable image, any size), `motion` (a Live
      Photo's video), `frames` (Apple's video frames: a flip-book), `meta` (.xmp,
      .aae). Apple: from the grouper; generic: from mime. Sizes: images from their
      header (JPEG, PNG, GIF, WebP), the main file from its metadata (the Photos DB
      first: already oriented).
- [ ] **S6. The asset contract in the API**: `/items` sends each asset by roles
      (original, edit, stills, motion, frames — url, mime, size); `/assets/:guid/:name`
      serves any file of that asset (and only of it). `PreviewPath` goes.
- [ ] **S7. The client decides**: the tile — `<picture>`/`srcset` from stills or the
      edit (the browser picks the size and the format: HEIC in Safari, JPEG
      elsewhere); hover — motion, or Apple's frames as a flip-book; click — the
      biggest still; "show the original" — open it, or download when the browser
      cannot show it.

### Dates and time zones

D1–D3 done (PR #13), see Done.

- [ ] **D4. API and client — to discuss.** Maybe not needed: if server and client
      normalise dates the same way, the API needs no separate zone. Sorting/grouping
      the gallery by date is separate (it makes all of this visible).

### Apple Photos library (`*.photoslibrary`) — its own milestone

A library is a database plus files named by asset UUID, not a folder of photos. The
truth is in `database/Photos.sqlite`; findings: "Apple Photos library: spike". The
grouper plugs into the import chain as the `groups/apple` branch (today a stub,
`groups.appleEnabled = false`). We only read — the DB and the files — never write.

- [x] **P0. Spike** (2026-09-30, on a copy of the dev library's DB + a file list):
      every file maps to a known asset; the layout and the local coverage are in
      findings.
- [ ] **P1. Grouper.** The first file of a library loads the asset links from the DB —
      from a copy (`Photos.sqlite` + `-wal`/`-shm` into a temp dir, then read):
      Photos may have it open and write it, and the library may be a network share
      or a copy (SQLite with WAL over SMB is unsafe); files arrive, a group
      goes out when all its local files are in (or at the end-of-walk marker). One
      group = one asset (GUID from the asset UUID): the original is the source (main
      file), the rest is linked to it (render, derivatives, Live Photo video, posters).
      The grouper states the main file; mime does not re-rank such a group. Trashed
      assets are not sent (their items go via deletions). `appleEnabled = true`, the
      generic `originals/`-only rule for libraries goes.
- [ ] **P2. What an asset shows** (decided):
      - **cloud-only assets are items** (Optimize Mac Storage: no local original);
        their preview is Apple's best derivative; metadata from the DB;
      - **an edited photo shows its edit** (the render; its JPEG derivative when the
        render is HEIC); the original stays the source;
      - **any size counts** — even the small `masters` thumbnail makes it Visible.
- [ ] **P3. Transcode: only the gaps.** Apple's derivatives are good JPEGs (~2000 and
      ~1000 px) — an asset with one is Ready from it, no transcode. Transcode only
      when the best derivative is below our size and the original is local, or when
      it is not browser-viewable (a HEIC render without its JPEG). Cloud-only: the
      best derivative is final. Videos (no local originals here): the `.THM` poster;
      the `cvt/…_tNNNN.jpeg` frames can play as a flip-book motion preview — no
      transcode. Apple's derivatives are a cache Photos may purge: we point at them,
      never copy; a vanished file falls back to the next best on the next walk.
- [ ] **P4. Library metadata** from the DB into the group as a virtual metadata record
      (e.g. `Photos:*` tags next to exiftool's) for the core plugins: date + zone
      (`ZTIMEZONEOFFSET`/`ZTIMEZONENAME` first in the date chain), GPS, dimensions,
      the original filename, favourite. Needed at once for cloud-only assets (no exif).
- [ ] **P5. Tests** on a fixture library (a minimal `Photos.sqlite` with the columns we
      read + files per the layout), including a Live Photo (its video is not local in
      the dev library: `<UUID>_3.mov` per osxphotos, unverified) and cloud-only assets.
- [ ] **Later: "show the original" for cloud-only assets.** We never write to the
      library — we ask Photos to download the original (PhotoKit, network access
      allowed); the next walk sees the original and reprocesses the group. PhotoKit
      exists only on the Mac that owns the library, in a user session — not in Docker
      (even on the same Mac), not on a NAS reading a share or a copy, not for an
      archived library. So it is an optional capability: a small macOS agent
      (launchd + a Swift helper) next to Photos, called by the server; the UI shows
      the button only when that agent is reachable; the grouper works on any copy
      without it. Needs the Photos privacy permission (not Full Disk Access). First a
      spike: does a PhotoKit request leave the original local in the library, or only
      hand the data to the caller? (AppleScript export copies the file out — not
      wanted.) Photos may purge the original again (Optimize Mac Storage): then the
      asset falls back to its derivative.
- Supported schema: `ZASSET` (macOS 11+); older (`ZGENERICASSET`) — not planned.
- Later: albums, people (`ZPERSON` / `ZDETECTEDFACE`) → perceptors.

### Then

- [ ] Thumbnails on the server: libvips via `bimg` (needs `brew install vips`), 400 px
      for tiles, 1600 px for the viewer, WebP; `/assets/:guid?size=…` falls back to the
      original; regenerated for `Dirty`, dropped for `Deleted`. Fixes blank tiles
      (decoding originals) and HEIC in Chrome/Firefox. Transcoders take the whole
      asset (group), not a file. Video: web previews are always downscaled (even a
      browser-playable H.264 can be 4K); codecs (H.264 / HEVC / AV1 support) decided
      then. Motion previews for videos and Live Photos: a short muted clip (or GIF)
      that plays on mouseover in the gallery, the poster otherwise.

      **Two-stage readiness** (decided): show what we can as early as possible, but
      never content the browser cannot show.
      ```
      validator -> exif plugins (date, size: fast, needed for the layout)
        -> cheap preview: what already exists, no transcode
             found  -> item Visible -> the client gets it
             none   -> the item waits for the expensive step
        -> expensive transcode (photo | video | Live Photo) -> item Ready -> client: updated
        -> ML perceptors (need thumbnails; results arrive as later updates)
      ```
      Cheap preview, in order: a browser-viewable derivative in the group (the JPEG
      of a RAW, the photo of a Live Photo, a Photos render); the embedded preview
      (`PreviewImage` / `JpgFromRaw`, `exiftool -b`); the original itself if the
      browser shows it (JPEG, PNG, WebP); a video's embedded poster. **Any size
      counts** — even a 160 px thumbnail: trust the data we have, the expensive step
      delivers the quality. States `New -> Visible -> Ready` (+ `Dirty`); `/items`
      shows `Visible` and `Ready`; `/assets/:guid?size=` serves the best that
      exists. Exif plugins move before transcode.

      **Two stages, two chains** (decided):
      ```
      cheap chain (as now):   walker -> ... -> exif plugins -> cheap preview -> Visible
                              -> perceptors, cheap pass (on what the group has)
                              the walker waits for this chain only, then the rescan pause
      expensive chain:        feeder (next item needing work, from the DB)
                              -> transcode (photo | video | Live Photo) -> Ready
                              -> perceptors, full pass (on our previews): results replace
                                 the cheap-pass ones, the client gets an update
      ```
      - **The expensive queue is DB state, not a channel** (a channel is a snapshot
        that cannot change): "items that still lack X" — `Visible`/`Dirty` without
        outputs; a perceptor whose stage/version for the item is behind. The feeder
        pulls the next one when a worker is free. New photos just appear in it,
        deleted items drop out of it, a move changes nothing (the path is read at
        pick time; outputs live under the GUID: `cache/thumbs/<guid>/…`).
      - An item being worked on may change meanwhile: at commit, deleted -> discard
        (long work checks between stages and stops); a different hash -> discard, the
        item stays queued; only the path changed -> keep.
      - Several workers: an "in work since" mark; stale marks go back to the queue.
      - Perceptors always run on both passes — incremental refinement for the
        client: e.g. faces on cheap previews find blurred spots that cluster as one
        face; on our previews part of them moves out into clusters of their own.
        Per item and perceptor we store the pass done (and the perceptor version).
        Order comes from the stages: every new item gets its cheap pass first.
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
- Reprocessing on plugin version change; fsnotify instead of the periodic walk.

### Core vs. perceptors

Rule: the server owns what the gallery cannot display correctly or cannot identify a
photo without, and what makes no sense to implement differently. Anything that adds
its own navigation axis or its own UI is a perceptor.

- **Core (exif_core):** date (+ time zone), size (+ orientation), media type, file
  identity and grouping, embedded preview, basic capture info (display only), user
  XMP/IPTC metadata (rating, tags, captions).
- **Perceptors:** geo and map, faces (`RegionInfo` as ready-made labels), objects,
  colour, navigation by camera/lens/exposure, series and bracketing, "similar photos".
