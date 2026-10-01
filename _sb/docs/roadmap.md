# Roadmap

Status as of 2026-10-01. Details and reasons — [findings.md](findings.md).
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
- Import chain as small steps (C1–C7, PR #14): fswalker only walks; groupers by
  source; a files gate (new/changed/never linked, deletions after every branch's
  marker); exif in parallel; mime ranks the main file (the source: RAW > video >
  image); validator as its own step; repeated walks (`rescan`) that never overlap.
  Graph: [`Import chain.puml`](../puml/Import%20chain.puml).
- Cheap stage (PR #15): items are `Visible` with what already exists — no transcode
  (the main file, a viewable derivative, an embedded preview; HEIC/HEVC without one
  wait). Apple Photos libraries: a grouper from a copy of `Photos.sqlite` (one group
  per asset, keyed by its UUID; trash/hidden skipped; a DB that cannot be read
  holds the library's files), the DB's date + zone, size, GPS, length and kind win
  over the files' EXIF. The asset contract: `/items` sends every file by role
  (original/edit/still/motion/frames, size, codec) + kind + length; the client
  chooses (`<picture>`/`srcset`, motion or Apple's frames on hover). Tiles: a badge
  (iCloud only, Live Photo / video, length); a video with no image is its own tile.
  Viewer: icon toolbar — Original/download (tested on open), autoplay for Live
  Photos and videos, stretch small images; switches kept per browser.
- Navigation (PR #16): **every perceptor is a view of the endless sheet** — its
  order ([`Perceptors.puml`](../puml/Perceptors.puml)). The base `perceplib`
  `Perceptor` has `View()` (title, SVG icon, help, relative) + `Order(anchor,
  items)` → entries with sections (level, label). The date is the default sheet
  (newest first, sections year → month in the zone of the shot); size (megapixels)
  and length (videos) are views too; the stubs geo and colour say they have no data
  yet. Config `perceptors.<name>`: `enabled` (run it), `client` (give it a button).
  `/items` streams newest first. API `GET /perceptors`, `GET /p/:name/order?anchor=`. The client lays out
  the order (layout stays client-side: it depends on the window); a switch keeps the
  photo the user is at in place and rearranges the rest — from the viewer, the
  viewed photo, centred. Perceptor buttons in the gallery toolbar and in the viewer;
  a side panel with the sections (scrubber; unpinned it shows over the photos while
  scrolling, pinned it takes its width from them). Viewer: ← → step through the
  sheet, Escape / a click close it (to the gallery even when `/N` was opened
  directly), the gallery shows the photo it closed on.

## Releases

- **0.2.0** — when the first perceptor works end to end (e.g. a primitive geo). Until
  then everything stays in `develop`.
- **First public release** — includes Docker.

## Next

One PR per feature (its steps are commits); docs are updated in that PR (AGENTS.md).

### Import chain — open

- [ ] Live Photo pairs checked by `ContentIdentifier` (today by name); an `animated`
      kind.

### Dates and time zones

D1–D3 done (PR #13), see Done.

- [ ] **D4. API and client — to discuss.** Maybe not needed: if server and client
      normalise dates the same way, the API needs no separate zone. Sorting/grouping
      the gallery by date is separate (it makes all of this visible).

### Apple Photos library — open

Done in the cheap stage (spike, grouper, what an asset shows, DB metadata, tests —
see Done and findings "Apple Photos library: spike"). We only read the library.

- [ ] **Transcode only the gaps.** Apple's derivatives are good JPEGs (~2000 and
      ~1000 px): transcode only when the best one is below our size and the original
      is local, or when nothing is browser-viewable (a HEIC render without its JPEG).
      Cloud-only: the best derivative is final. Apple's derivatives are a cache Photos
      may purge: we point at them, never copy.
- [ ] **"Show the original" for cloud-only assets** — most of an iCloud library
      (here: 3 528 of 5 866 photos, 766 of 770 videos, all 351 Live Photos). We never
      write to the library — we ask Photos to download the original (PhotoKit,
      `PHAssetResourceManager`, network access allowed); the next walk sees it and
      reprocesses the group. PhotoKit exists only on the Mac that owns the library,
      in a user session (not Docker, not a NAS reading a share or a copy). So an
      optional capability: a small Swift helper (CLI first, later a launchd agent with
      the Photos permission), called by the server; the viewer's Original button asks
      it when the original is not local. First a spike: does the request leave the
      original in the library, or only hand the data to the caller? Photos may purge
      it again (Optimize Mac Storage): then the asset falls back to its derivative.
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

- [ ] Optimal (Dijkstra) layout for an already loaded gallery — optional.

## Later: the perceptor platform

Don't rush — a stable core first.

- Server → client events (New / Updated / Processed Item) as designed in
  [`Client flow.puml`](../puml/Client%20flow.puml) (MQTT): the client draws a
  preloader, fixes the grid, then the thumbnail. Until then — incremental
  `/items?since=` + tombstones as a fallback.
- ML as a separate service (goMLer) per [`ML Flow.puml`](../puml/ML%20Flow.puml):
  consumes Processed Item, runs ML plugins, returns metadata.
- Perceptor data: the core keeps it, a perceptor only declares it — see "Perceptor
  data" below.
- Navigation is a base requirement of a perceptor (decided): every perceptor gives
  the sheet an order — no filters, the project is one endless sheet in different slices. Absolute
  ones ignore the anchor; relative ones (faces, objects, similar) build a
  **two-sided trail** from it: from the anchor to the nearest unseen photo, then the
  nearest to that, in both directions. Next:
  - [ ] **Perceptor data storage + geo** (first: the data comes from EXIF and the
        Photos DB, no pixels — the simplest test of the storage; design: "Perceptor
        data" below). Geo: (lat, lon) on a Hilbert curve (one dimension that keeps near
        places near; a plain longitude puts Krakow next to Cape Town), sections
        country → city.
  - [ ] **Colour — deterministic, no ML** (`ml_color` → `color`). Needs pixels for
        perceptors: the item's cheap preview (always a browser image — JPEG/PNG/
        WebP, so no HEIC/RAW decoding), scaled to ~32×32 (~10–20 ms per photo, once
        per photo). Per photo: a colour histogram in a perceptual space (OKLab /
        CIELAB, ~64 bins), 2–3 dominant colours (median cut — deterministic, unlike
        k-means), mean lightness and chroma (greys apart). Views (one order per
        perceptor, so two perceptors if both):
        - **rainbow** (absolute): by the dominant hue around the circle, sections
          named by the colours actually present; greys apart, dark → light;
        - **similar colours** (relative): the trail by histogram distance (ties by
          guid).
        To decide: rainbow, trail or both.
  - [ ] Relative perceptors (the trail) with the first ML perceptor (faces): the
        trail mechanism shared with the colour trail.
  - [ ] A stable link to a photo: the viewer's `/N` is a position in the current
        sheet and changes with the perceptor.
- UI slots (`item-panel`, `view`) — a perceptor can bring its own UI (map, face
  management).
- Reprocessing on plugin version change; fsnotify instead of the periodic walk.

### Perceptor data (design, decided 2026-10-01)

Diagram: [`Perceptor data.puml`](../puml/Perceptor%20data.puml). Why not the other
ways — findings, "Perceptor data: the core keeps it".

**Per-item values (`ProcessingMode` Single) — next, with geo:**

- A perceptor **declares** its data as a Go struct; the core creates, migrates and
  maintains the storage. The plugin never sees SQL or a driver.
  ```go
  // a package of the plugin's own (not main): another perceptor may read it
  type Location struct {
      Lat float64
      Lon float64
  }

  var Places = api.NewStore[Location]("geo", 1) // name, schema version

  func (p *geoPerceptor) Schema() api.Schema { return Places.Schema() }

  // the processor (Decorate): put the value on the item
  Places.Put(in, Location{Lat: lat, Lon: lon})

  // Order: read the values the core loaded for the items
  loc, ok := Places.Get(it)
  ```
- **Typed API**: `Store[T]` with `Put(item, T)` / `Get(item) (T, bool)` — no string
  keys or `any` outside `perceplib` (a `SetValue("lat", …)` was rejected: untyped and
  too generic). Columns come from T's fields by reflection, once, at registration:
  `float64`, `int64`, `string`, `bool`, `time.Time`, `[]float32` (a vector; its size
  from a tag, `perceptor:"dim=512"`).
- **Writing**: `Put` only puts the value on the item in the chain; the core commits
  it at the end (the closer), together with the item.
- **Reading**: the core loads the perceptor's values for the items it passes to
  `Order` (by guid).
- **Rows**: keyed by the item's GUID (stable across moves; Apple: the asset UUID) +
  the schema version + the pass (cheap / full, see "Two stages").
- **Kept by the core**: an item deleted → its rows in every perceptor's storage go;
  a schema version changed → the perceptor's data is rebuilt and its items are queued
  for it again.
- **Physically**: SQLite — a file per perceptor, `data_dir/perceptors/<name>.db`
  (SQLite has one writer per file: an ML perceptor writing embeddings does not block
  the import; a perceptor's data is reset by deleting its file; no `ATTACH` — the
  core reads by a list of guids, no cross-file joins). Postgres — one database, a
  schema per perceptor (`perceptor_geo`): one backup, the same split.
- **Vectors** (embeddings, colour histograms): stored as blobs; the trail's nearest
  neighbours from an in-memory index (HNSW) built at load — later `sqlite-vec` /
  `pgvector`. `Vector` is a kind of its own so the backend can change without the
  plugins knowing.
- **Reading another perceptor's data**: the struct lives in an importable package
  of that perceptor, and a reader declares the dependency (journeys need geo).

**Groups (`ProcessingMode` Group) — later, not in work yet:**

- Single perceptors work photo by photo (all of today's). Group perceptors produce
  **groups of photos with data of their own**: face clusters, album suggestions,
  journeys (by time and place), series, duplicates.
- Typed the same way: the group's data is a struct (a journey: title, period,
  place); storage — a table of groups and one of "photo → group", kept by the core
  under the same rules (deletions, schema version), in the perceptor's file / schema.
  ```go
  var Trips = api.NewGroups[Trip]("journey", 1)

  func (p *journeyPerceptor) Group(ctx context.Context, items []api.ItemDataProvider) error {
      // … cluster by time and place (reads geo.Places)
      Trips.Put(Trip{…}, guids)
  }
  ```
- **Not in the import chain**: a group cannot be decided one photo at a time. A pass
  of its own over the library (or what changed) after the import — the expensive
  queue as DB state (see "Two stages").
- **Groups are sections**: a group perceptor's `Order` gives the sheet by its groups
  ("Lviv, May 2025"), photos in each by time; the side panel is the list of groups.

### Core vs. perceptors

Rule: the server owns what the gallery cannot display correctly or cannot identify a
photo without, and what makes no sense to implement differently. Anything that adds
its own navigation axis or its own UI is a perceptor.

- **Core (exif_core):** date (+ time zone), size (+ orientation), media type, file
  identity and grouping, embedded preview, basic capture info (display only), user
  XMP/IPTC metadata (rating, tags, captions).
- **Perceptors:** geo and map, faces (`RegionInfo` as ready-made labels), objects,
  colour, navigation by camera/lens/exposure, series and bracketing, "similar photos".
