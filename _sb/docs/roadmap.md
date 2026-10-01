# Roadmap

Status as of 2026-10-01 (PR #17). Details and reasons — [findings.md](findings.md).
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
- Perceptor data (PR #17): a perceptor declares its data as a struct
  (`api.NewStore[T]`, typed `Put` / `Get`); the core keeps it — SQLite, a file per
  perceptor in `data_dir/perceptors/` (Postgres: not yet). Values are committed with
  the item; an item a perceptor has no row for is processed again; gone items are
  pruned after a walk. Geo is its first user: coordinates from EXIF or the Photos DB,
  the sheet on a Hilbert curve with every city and region in one piece, sections
  region → city from the time zone. A photo may start a path of sections; the side
  panel's scale is by sections, √ of their photos, at every level
  ([`Perceptor data.puml`](../puml/Perceptor%20data.puml)).

## Releases

- **0.2.0 — the first release**: transcode (photos and video, hardware video
  encoding) and Docker. A release means an image someone installs; without the
  transcode the system is not complete (HEIC in Chrome, iPhone HEVC video, big
  originals in the grid). The first perceptor end to end (geo, PR #17) is no longer
  the bar. Until then everything stays in `develop`.

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

- [ ] **The expensive stage: previews and transcode** — design below ("Expensive
      stage"). Regenerated for `Dirty`, dropped for `Deleted`. Transcoders take the
      whole asset (group), not a file. Motion previews for videos and Live Photos: a
      short muted clip that plays on mouseover, the poster otherwise.

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
- [x] First perceptor end to end — geo (PR #17): its data, its view of the sheet.
- [ ] Geo on a map (markers) — a perceptor UI slot (see "Later").

## Core — product (gontroller)

- Own media-type table and the main file of a group → **C5** (import chain).
- [ ] One main item merged from all sidecars
      ([`Item flow.puml`](../puml/Item%20flow.puml)).
- [ ] Embedded RAW preview (`PreviewImage`/`JpgFromRaw`) for the transcoder.

## Core — service (gontroller)

- [x] Broken files (PR #18): a generic group whose main file exiftool reports as broken
      (`Error`: "File format error", "File is empty") or an image with no size at all
      (a JPEG cut after its header) is not an item; its files are ignored until one
      changes (no retry on every walk); a photo that gets corrupted loses its item.
      Apple assets are not judged by their file. A group exiftool returns nothing for
      at all is still retried (that may be a passing failure).
- [x] Fewer Info logs in `fswalker` — none per file since the import chain (C1).
- [ ] `TestLoadExternalPlugins` should load real `.so` files.

## Deployment (first release)

- [x] **Debug / release mode in the config** (PR #18): `mode: debug | release`
      (release by default, debug in the dev config); `GET /app` gives the client the
      version and the mode, so one client build serves both. Release: server logs at
      Info (no SQL — it is logged at Debug), no per-request lines; the client logs
      warnings and errors only (the workers get the mode as a message), no tile
      borders, a neutral placeholder instead of "Can't render item".

- [ ] Dockerfile (with 0.2.0): CGO (sqlite, libvips), jellyfin-ffmpeg, exiftool from a
      `dist-*` release, fix `CMD` (`--config /data/config.yml`, `/data` as a volume =
      what `.var/` is in dev). A base compose that runs anywhere (software encoding)
      + an override per accelerator (`hwaccel.qsv.yml`: `/dev/dri` and the `render`
      group; `hwaccel.nvenc.yml`: NVIDIA Container Toolkit).

## Frontend

- [ ] **A direct link to a photo waits for the sync**: every page load starts the
      `/items` sync from zero (itemsDb is cleared), and the viewer shows a photo once
      the stream reaches it and the layout places it — up to tens of seconds on a big
      library; the side panel's marks come in the same way (some are missing until
      their photos are placed). Keep the synced items between loads (sync the
      changes), or fetch the linked photo first.

- [x] **Item info panel** in the viewer (PR #18): a toolbar toggle, can be pinned open
      (kept in the browser). Its content comes from the perceptors: each one gives what it knows
      about the item — a base requirement next to navigation (`Perceptor.Info(item)` →
      fields with labels and values; `GET /items/:guid/info` → per perceptor: its title,
      icon and fields). Today: date (the date, its zone and where it came from), size
      (pixels, megapixels), length, place (coordinates, region / city), the asset's
      files (original, edits, what is local / in iCloud).
- [x] **A burger menu instead of the side panel's pin** (PR #18): a dropdown with
      toggles — pin the side panel; show / hide each perceptor's button. Three levels for a perceptor:
      **off** (config `enabled: false` — not run), **hidden by the server** (`client:
      false` — runs, the client does not get it), **hidden by the client** (the user's
      toggle in the menu, kept in the browser).

- [x] **Views and photos have URLs** (PR #18): `/v/<view>` — the sheet in a view,
      `?at=<guid>` — around that photo (kept up to date as you scroll: a reload or a
      shared link opens the same place); `/v/<view>/<guid>` — the viewer on a photo,
      ← → walking that view. `<view>` is the perceptor's public slug (`api.View.Slug`:
      `date`, `size`, `length`, `place`, `colour`; unique — a taken one is not given to
      the client), not its plugin name. `/` redirects to `/v/date`; an unknown view goes
      to the first. A switch is a history entry: Back returns to the previous view
      around the same photo. The URL is the state (the view is no longer kept in the
      browser's storage).
- [x] Side panel: the first section of a deeper level (the first month of a year, the
      first city of a region) sits at the same point as its parent's label — its
      label now goes just under the parent's when there is room (PR #18).
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
  - [x] **Perceptor data storage + geo** (PR #17; design: "Perceptor data" below).
        Geo keeps the coordinates (EXIF or the Photos DB's record); the sheet on a
        Hilbert curve (a plain longitude puts Krakow next to Cape Town); sections
        from the time zone of the place — region → city ("Europe", "Kyiv").
  - [ ] Geo sections by country / city names (needs a geocoder dataset, e.g.
        Natural Earth / GeoNames offline) instead of the time zone's.
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
  - [x] A stable link to a photo — `/v/<view>/<guid>` (see Frontend).
- UI slots (`item-panel`, `view`) — a perceptor can bring its own UI (map, face
  management).
- Reprocessing on plugin version change; fsnotify instead of the periodic walk.

### Expensive stage: previews and transcode (design, 2026-10-01)

The transcode belongs to the core (as in Immich, PhotoPrism, Jellyfin: previews and
ffmpeg in the server; ML apart — gomler). Its queue is DB state ("Two stages"), so any
process with the database and the files can take work.

**Photos** — libvips on the CPU (a GPU gives nothing here); HEIC via libheif, RAW via
its embedded preview (or libraw).
- **Sizes: an array in the config**, long side px — the system takes any array.
  Default `[400, 1600]` to start with (tiles, the viewer); previews, not copies of the
  original: ~3840 is close to the original itself. Worth trying later: 800 (tiles
  are ~400 CSS px — 800 on Retina) and 2560 (pixel-perfect on a 2K 32"; a MacBook
  16" is 3456×2234). `srcset` picks the size and the density; the original stays
  behind the viewer's Original switch.
- **Format: one, chosen in the config** — `webp` (default) or `avif`:

  | | WebP | AVIF |
  |---|---|---|
  | size at the same quality | base | ~20–30% smaller |
  | gradients (sky, skin) | banding possible | cleaner |
  | depth, HDR | 8 bit, no HDR | 10–12 bit, HDR, wide gamut (iPhone Display P3) |
  | encoding | fast | 5–10× slower on the CPU |
  | browsers | all | all current (Safari 16+) |

**Video** — ffmpeg (**jellyfin-ffmpeg**: every hardware backend and HDR tone mapping
in one build, as Immich does).
- **Codec: one, chosen in the config** (`h264` default — plays everywhere; `hevc`,
  `av1`). The core maps it to the accelerator's encoder (`h264` → `h264_qsv` /
  `h264_vaapi` / `h264_nvenc` / `h264_videotoolbox` / `libx264`).
- **Accelerator in the config**: `auto | none | qsv | vaapi | nvenc | videotoolbox`.
  At start a probe (a few frames of `testsrc`) checks it; a failure is logged and the
  software encoder is used.
- **The whole chain on the GPU** — decode → scale → encode without copying frames to
  the CPU (`-hwaccel qsv -hwaccel_output_format qsv`, `scale_qsv`): that is where the
  speed comes from.
- **HDR → SDR tone mapping** — iPhone video is HLG / Dolby Vision HEVC: without it an
  H.264 copy comes out washed out (`vpp_qsv` / `tonemap_opencl` / `libplacebo`). To
  be checked on real iPhone videos.
- Outputs: the viewer's video (sizes from the config, like photos), a short muted
  hover clip, a poster.
- **Hardware**:

  | | encode | decode | in Docker |
  |---|---|---|---|
  | Intel QSV / VAAPI (dev box: i5-13500T, UHD 770) | H.264, HEVC 8/10 bit; AV1 only on Arc / Core Ultra | + AV1 | `/dev/dri`, `render` group |
  | NVIDIA NVENC | H.264, HEVC; AV1 on Ada+ | + AV1 | NVIDIA Container Toolkit |
  | AMD VAAPI | H.264, HEVC | | `/dev/dri` |
  | Apple VideoToolbox | H.264, HEVC | | no: Docker on a Mac has no GPU — run the binary natively |

  On the dev box AV1 encoding is software only (SVT-AV1, slow): `h264` / `hevc` there.

**Where it runs**: in gontroller by default — one compose, one process. The same
binary may later run in roles (`role: transcoder`, as Immich's workers) on a GPU box,
taking work from the same DB queue; nothing extra to design for it — the queue is
already DB state.

- [ ] **Later: several codecs / formats at once** (`codecs: [h264, av1]`, `formats:
      [avif, webp]`) — the asset contract already sends each rendition with its codec
      and `<picture>` / `<source type>` lets the browser pick the best it plays. But
      each one is another encode: N× the disk and the transcode time, so it is a
      choice of its own, not the default.

### Perceptor data (decided 2026-10-01; Single done in PR #17)

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
- **Rows**: keyed by the item's GUID (stable across moves; Apple: the asset UUID);
  `has = 0` records "processed, nothing found" (no GPS), so such an item is not
  processed again on every walk. The pass (cheap / full) comes with the expensive
  stage (see "Two stages").
- **Kept by the core**: rows of gone items are pruned after every complete walk; a
  changed schema (version or fields) drops the perceptor's values, and the files
  gate processes an item again while an import perceptor has no row for it (new
  perceptors and schema changes take one pass over the library).
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
