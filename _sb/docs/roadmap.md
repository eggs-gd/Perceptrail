# Roadmap

Status as of 2026-10-02 (PR #21). Details and reasons — [findings.md](findings.md).
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
- Views by URL, info panel, delta sync, debug/release mode (PR #18); the roadmap's
  old items sorted, the plugin loader test on real `.so` files, RAW previews keep
  their orientation (PR #19); several tabs — one sync at a time, a self-healing copy,
  a layout per tab, plain HTTP on a LAN address (PR #20); providers listed (PR #22).
- **Apple Photos on demand** (PR #21, transcode step 0): Photos, asked through
  PhotoKit, makes a cloud-only rendition local in its own library — we render and
  store nothing it keeps. `pkg/photokit` (cgo, macOS); `/items/:guid/rendition/
  {medium,hover,original}`; one asset processed again without a walk (`Regroup` →
  the gate → `Refresh`); assets with nothing local asked for in the background;
  `/items/:guid/files`. The viewer opens on what is here (the comfortable ~2048 px,
  or the full size lit as the Original), the medium asked for only when nothing here
  is that big; hover with a loading ring; the tile's cloud = the full resolution is
  not here; Info → Files with downloads. The sync takes a new contract in place and
  passes nested changes to the layout. Spike and results: `_sb/spikes/photokit`,
  findings "PhotoKit spike".

## Releases

- **0.2.0 — the first release**: transcode (photos and video, hardware video
  encoding) and Docker. A release means an image someone installs; without the
  transcode the system is not complete (HEIC in Chrome, iPhone HEVC video, big
  originals in the grid). The first perceptor end to end (geo, PR #17) is no longer
  the bar. Until then everything stays in `develop`.

## Next

One PR per feature (its steps are commits); docs are updated in that PR (AGENTS.md).

### Providers — other libraries as sources (2026-10-02, to work out)

The Apple step (PhotoKit, PR #21) showed the shape: a library that already keeps
renditions is a **provider** — we read its assets and metadata, and ask it for a
rendition when the client needs one; we render nothing it already has. Perceptrail
stays a viewer over the user's library (findings "A viewer over a library, not a
library").

- **Our client knows only our server.** The provider is server-side: gontroller
  talks to the library's API with its key and passes the bytes on (a proxy) — the
  key and the library's address never reach the browser, and the library need not
  be reachable from the client's network. The client contract is the one PR #21
  brought: `asset.onDemand` (medium, hover, original — versioned URLs), `asset.full`
  and `/items/:guid/rendition/...`, the same for every provider; the viewer picks
  from what is here first.
- **No access to their databases** where there is an API (Immich and PhotoPrism keep
  Postgres / MariaDB with schemas that change between versions); a database or a
  catalog only where it is the library's own format (Apple, Lightroom, digiKam).
- **Their metadata as ours**: the provider's record goes into the group with
  exiftool's tag names (as the Photos DB does), so the perceptors read it unchanged;
  their faces, people, albums, labels, smart search — perceptor data without an ML of
  our own.
- A cloud-only original is not described (its name, format, weight): for local
  files the original is what is on disk; a provider's own description waits for a
  second provider — the abstraction comes from two or more, not from one.
- [x] **The mechanism** (PR #23): `pkg/providers` — one switch sends a found file to
      the grouper of the first provider that claims it (the plain folder,
      `providers/folder`, last), and the item's provider gives on-demand renditions;
      Apple Photos is the first library (`pkg/providers/apple`), enabled by the
      config. A provider that comes later
      takes its files over through the usual deletions (findings "Providers as steps
      of the chain").
- [x] Generalise `rendition.go` (PR #23): the provider gives the rendition (Apple: the
  local file or PhotoKit; an API provider: its thumbnail / preview / playback,
  proxied), the web service serves it.

All of them listed for now; the order is to be decided:

| provider | how | what it gives | notes |
|---|---|---|---|
| **Apple Photos** | `Photos.sqlite` + files; PhotoKit on demand | renditions, edits, Live Photos, video renditions | done (reading; on demand: PR #21) |
| **Immich** | REST API + an API key (`asset.read`, `asset.view`; `asset.download` only for originals); Sync v2 (streamed, resumable deltas) | `thumbnail` / `preview` (~1440 px) / original, transcoded video playback; faces, people, albums, CLIP search | **first** — the owner uses it daily. To check: API stability between versions, Sync v2 from a non-mobile client, its video transcode policy |
| **PhotoPrism** | REST API + an app password (Bearer) | thumbnails `/api/v1/t/<hash>/<preview token>/<size>`, H.264 video; labels, faces, places | similar to Immich |
| **Lightroom Classic** | the catalog `.lrcat` (SQLite) + the previews `.lrdata` | ratings, keywords, collections, edits; its previews | the previews' format is Adobe's own; fits "the asset from all its files" |
| **digiKam** | its SQLite / MySQL DB + the files | tags, faces, ratings | no server |
| **Nextcloud Memories** | WebDAV / Memories' API | previews, albums, faces (with Recognize) | |
| **LibrePhotos** | REST API | faces, places | less alive |
| **Synology Photos** | DSM's API (unofficial) | many NAS users | the API is not stable |
| **Ente** | its own export (desktop app, `ente export` CLI): decrypted files + metadata JSON | a folder with its sidecars | no API for us; end-to-end encrypted otherwise |
| **Google Takeout** | the archive: folders + `*.supplemental-metadata.json` per file (taken time, GPS, description; the files' EXIF often stripped) | a folder with its sidecars | "the asset from all its files"; to move it into Immich there is `immich-go` (simulot/immich-go) |
| Google Photos (live) | — | — | not possible: since 2025 its API sees only what the app itself created |

### The asset from all its files — open

Perceptrail is not a library of its own but a viewer over a popular one: Immich,
PhotoPrism, Lightroom, digiKam, Apple Photos keep the albums, tags, sync and the heavy
UX; we give another way through the same photos — rediscovery. Most of them write
sidecars (by default or on request), so an asset is a package — the original, RAW +
JPEG, edits, `.xmp` / `.aae`, the motion part — and the grouping exists to collect
the whole package, not only to find its main file.

- [ ] **One item from the whole package** — the metadata merged from every file of
      the group, not only the main file's: rating, tags, captions, face regions from
      the XMP sidecars those libraries write ([`Item flow.puml`](../puml/Item%20flow.puml)
      — designed that way, not finished). Which library writes what, and what wins
      when files disagree — to work out per library.
- [ ] **Live Photo pairs by `ContentIdentifier`** in generic folders (today by name:
      a rename breaks the pair, two namesakes stick together); Apple libraries pair
      from their DB already. An `animated` kind (GIF, animated WebP / HEIC) shown
      moving, not as a still.

### Dates and time zones

D1–D3 done (PR #13), see Done. D4 (a zone in the API) — parked: the order and the
sections come from the date perceptor on the server, the info panel gets the date
already formatted in the shot's zone. Keep it only when a UX reason for the zone
shows up (the place already tells where it was taken).

### Apple Photos library — open

Done in the cheap stage (spike, grouper, what an asset shows, DB metadata, tests —
see Done and findings "Apple Photos library: spike"), and on demand (PR #21 — see
Done). We only read the library.

- [x] **Transcode only the gaps** — none for a Photos library: whatever the client
      needs, Photos makes local on request (PR #21); our transcode is for generic
      folders (steps 1–2).
- [x] **"Show the original" for cloud-only assets** — the Original from Photos (PR
      #21): the biggest of what is seen, at full resolution.
- [ ] The Photos permission when gontroller is not started from a terminal
      (launchd) — the binary would need its own (Info.plist, a stable signature).
- Supported schema: `ZASSET` (macOS 11+); older (`ZGENERICASSET`) — not planned.
- Later: albums, people (`ZPERSON` / `ZDETECTEDFACE`) → perceptors.

### Then

- [ ] **The expensive stage: previews and transcode** — design below ("Expensive
      stage"). In steps, each its own PR (the cut may still change):
      0. ✅ **Apple Photos first — a spike, then a fork** (done: PR #21; the dev library is mostly
         iCloud-only, and Photos' DB knows every asset's renditions —
         `ZINTERNALRESOURCE`: recipe, size, local / in iCloud; recipe 65741, up to
         ~2048 px, is in iCloud for nearly every asset — 3 240 have it only there).
         - **The principle: one copy.** Whatever Photos keeps, we do not keep again —
           a cache of our own beside the library does not save disk, it adds a
           second copy. What is local we serve as it is; what is not we ask Photos for
           (PhotoKit), and Photos stores it in its own library, under its own
           storage policy (Optimize Mac Storage purges it when space runs low — we
           then fall back to a smaller rendition and ask again). The most we may
           keep: a bounded working set (an LRU with a size limit, e.g. 2 GB) so the
           viewer's neighbours open at once — any entry may vanish, it is not a copy
           of the library. Our own renditions only for what Photos does not have
           (generic folders; formats a browser cannot show).
         - **By level, on demand** — not "make everything medium": recipe 65741
           averages ~1 MB (~2.6 bits/pixel — a high-quality JPEG, Photos shows and
           edits from it), ~100 GB per 100 k photos, ~40 % of the originals:

           | for | rendition | when |
           |---|---|---|
           | tiles | small (65743 360×480 ~76 KB, 65747 768×1024 ~300 KB — often local already) | ahead, in the background |
           | the viewer | medium, ~2048 | when a photo opens, plus its neighbours |
           | the original | the full file | the Original button |

           (Our own webp at 1600 px would be ~250 KB — fewer pixels ×0.6, quality
           ~q80 ×0.6, the codec ×0.7 — but that saves space on a copy beside theirs:
           not an argument.)
         - **"Download Originals to this Mac" — supported, not the only way.** For
           most libraries (the dev one too) it is the simplest: one checkbox,
           everything local, nothing to ask Photos for. But people with decades of
           photos (hundreds of thousands) would be forced to sync the whole library —
           and Photos has no "keep renditions, originals on request" mode: it is
           either everything or renditions at its own discretion. For them (Optimize
           Mac Storage) asking Photos per level is exactly that missing mode, and the
           reason to have the PhotoKit step at all. The docs give both: the checkbox
           for who can afford the disk, the step for who cannot.
         - **Go, not Swift.** PhotoKit is Objective-C: Go calls it through cgo (an
           `.m` file beside the Go code), in the gontroller process — no second
           process, no second language. cgo is there already (Go plugins need it);
           the code sits behind `//go:build darwin`, the Linux build has no Apple
           step. The Photos permission needs `NSPhotoLibraryUsageDescription`: a bare
           binary gets an Info.plist through the linker
           (`-sectcreate __TEXT __info_plist`). To check: who the permission is
           granted to when started from a terminal (the binary or the terminal), and
           whether each rebuild (a new ad-hoc signature) drops it — if it does,
           development needs a stable signature, or the PhotoKit part becomes a small
           process of its own after all.
         - **Where it runs.** Reading the library (`groups/apple`: a copy of
           `Photos.sqlite` + the files on disk) is pure Go and stays in every build:
           an archived library after leaving Apple, a library on an external disk, a
           copy on a NAS — what is on disk is shown, what is missing is marked.
           Asking Photos is the native macOS build only. **Docker on a Mac** (Docker
           Desktop is a Linux VM) has no PhotoKit, and macOS does not let it into a
           `.photoslibrary` without Full Disk Access: a Mac with Photos runs the native
           binary, Docker is for servers — the Docker step says so in its docs.
         - [x] **spike** (Go + cgo, `_sb/spikes/photokit`; findings "PhotoKit spike"):
           **Photos normalises the library.** Asked for a cloud-only asset's image
           (≤ 2048 px, network allowed), it downloads recipe 65741 (1536×2048,
           ~0.8–0.96 MB) into `resources/derivatives/<X>/<UUID>_1_102_o.jpeg` and marks
           it local in the DB; the original stays in iCloud. 0.6–0.9 s per photo; "is
           it local" answers at once (error 3164 without network). The permission goes
           to the terminal that starts the binary, not to the binary.
         - [x] **→ the Apple step** (the fork taken; done in PR #21 — the spike and
           its implementation in one):
           nothing rendered or stored by us, nothing written to the library by us —
           Photos downloads, the walk finds the file (the grouper already knows
           `_1_102_o.jpeg`). Only on demand, never in bulk (decided):
           - **the sheet** asks for nothing: Photos keeps
             `masters/<X>/<UUID>_4_5005_c.jpeg` (~100 KB) local for nearly every
             asset, cloud-only ones too. The exception (Codex): an asset with nothing
             local at all (7 of 6 427 here) is Waiting — never on the sheet, never
             opened — so those are asked for in the background, each once per run;
           - **opening a photo** triggers it: the viewer shows what there is (the
             tile's image) at once, the server asks Photos for the medium rendition
             (~1 s) — the request waits for it and serves the file, the viewer swaps
             the image in; the asset's group is processed again, so the item and the
             other tabs learn of it through the delta;
           - **the neighbours** — 1–3 rows around the opened photo (in the view's
             order) are asked for ahead, so the arrows open at once; not more;
           - **a video on the sheet** (decided): `fast` is enough — H.264 360p,
             ~0.7 MB, ~0.9 s — asked for on hover; meanwhile Photos' `cvt` frames if
             it has them. **Hover prefers the video**: once the 360p is local it
             plays, the frames are not shown;
           - **a video in the viewer**: `medium` on open, as a photo's medium
             rendition — HEVC or H.264 720p. HEVC plays in every current browser
             with a hardware decoder (findings "HEVC in browsers, 2026"); the rare
             one without (Chrome on Linux, old hardware) gets the `fast` 360p — the
             client decides (`canPlayType('video/mp4; codecs="hvc1"')`), the asset
             contract already sends each file's codec. Never `automatic` / `high`:
             they download the original. No H.264 720p of our own (decided);
           - the server's cheap stage counts only H.264 as browser-playable (HEVC
             waits for a transcode): let the client decide by `canPlayType`
             instead;
           - **a Live Photo** (decided): as a video on the sheet — its motion is
             asked for on hover (`requestLivePhotoForAsset`, H.264 ~650×870,
             ~1.8 MB, ~0.9 s) and plays there once local; the same file in the
             viewer;
           - the grouper learns the video renditions' names (`_2_3_o.mp4`,
             `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`).
           Done as: `pkg/photokit` (cgo, macOS; a stub elsewhere; the main thread
           turns the main run loop), `GET /items/:guid/rendition/{medium,hover}`
           (serves the local file, asks Photos when there is none: one request per
           asset and level, three at once), `asset.onDemand` for Photos items; the
           client lays the medium over the image, plays the hover video after 250 ms,
           asks for ±6 neighbours ahead. Access is asked for only when a Photos
           library is under the root.
         - [x] **The Original** (PR #21): the biggest of what the user sees — a
           photo's (a Live Photo's photo's) current version, the edit, at full
           resolution, drawn by Photos as JPEG (any browser shows it, HEIC in Chrome
           too); a video's original file (high quality). Edits are Photos' business:
           no unedited original, no provider specifics. The viewer's switch for all of
           them; the tile's cloud means "the original is only in iCloud" for every
           kind now.
         - [ ] Still open: the permission when not started from a terminal
           (launchd). Asking for thousands in a row is not needed (nothing asks in bulk).
      1. **photo renditions** — libvips on the CPU, the source chosen to avoid a full
         decode (Photos' JPEG, the HEIC's embedded thumbnail, a RAW's embedded JPEG —
         `PreviewImage` / `JpgFromRaw`), the DB-state queue, benchmarks on the real
         library (generic folders, and Apple assets that still lack a viewable size);
      2. **video, software** — `libx264`, HDR → SDR, hover clip, poster; the codec →
         encoder table and the probe with a software fallback from day one;
      3. **Docker** — the image (jellyfin-ffmpeg), a base compose with software encoding;
         the docs say a Mac with Photos runs the native binary (no PhotoKit in Docker);
      4. **hardware acceleration** — video: QSV (`hwaccel.qsv.yml`, the i5),
         VideoToolbox (native on a Mac), NVENC when there is one to test on; photos: a
         macOS ImageIO decoder for HEIC only if the benchmarks show HEIC is the
         bottleneck.

      0.2.0 needs 1–3; 4 is wanted, not required. Regenerated for `Dirty`, dropped for `Deleted`. Transcoders take the
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
- [ ] One main item merged from all sidecars → "The asset from all its files" (Next).
- [x] Embedded RAW preview (`JpgFromRaw` / `PreviewImage`) — the cheap preview has
      extracted it since PR #15; it now gets the RAW's Orientation (portrait shots
      lay on their side). The renditions step uses it as a source too.

## Core — service (gontroller)

- [x] Broken files (PR #18): a generic group whose main file exiftool reports as broken
      (`Error`: "File format error", "File is empty") or an image with no size at all
      (a JPEG cut after its header) is not an item; its files are ignored until one
      changes (no retry on every walk); a photo that gets corrupted loses its item.
      Apple assets are not judged by their file. A group exiftool returns nothing for
      at all is still retried (that may be a passing failure).
- [x] Fewer Info logs in `fswalker` — none per file since the import chain (C1).
- [x] **`TestLoadExternalPlugins` loads real `.so` files**: every perceptor in
      `perceptors/` (stubs aside) is built with the test's own Go and loaded with
      `plugin.Open` — a plugin built against other dependency versions than the host
      (it happened: `x/sync`, `testify`) fails it. CI runs it uncached.

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

- [x] **Kept items, a delta sync** (PR #18): a direct link and the side panel's marks
      waited for a sync from zero on every load (itemsDb was cleared). The items are
      kept between visits; `/items?since=<cursor>` brings only what changed — changed
      items, and `{guid, removed}` for the deleted or hidden ones; the server's epoch
      (its database) decides when a full sync is needed. The layout starts from the kept
      items and the view's kept order (then the server's, applied only if it differs).
      Refreshed on the page's own moments: the start, coming back to the tab, every
      navigation (at most every 5 s). A reload: tiles and marks in ~0.2 s, a direct
      link ~0.3 s; an empty delta is 0 bytes instead of 3.5 MB. Several tabs: one sync at a time (Web Locks), and a copy
      whose count differs from the server's `total` heals itself with a full sync. The layout
      is per tab (its own database: width and view are the tab's).

- [x] **Item info panel** in the viewer (PR #18): a toolbar switch, kept like the
      other viewer switches (open photo to photo and between visits, until switched
      off — a separate pin inside the panel was not obvious and is gone). Its content comes from the perceptors: each one gives what it knows
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
- [ ] **Zoom in the viewer** — the wheel zoom (scale of the stage around its centre)
      worked badly and is gone (PR #21). To do it properly: look at how Immich and
      Google Photos do it (they differ) — around the pointer, pan when zoomed, pinch
      on a trackpad, double-click, and the original's pixels when zoomed past the
      preview.
- Maybe, some day: the optimal (Dijkstra) layout. Where the gallery started: the
  best row breaks over the whole set (rows closest to the target height). It keeps
  the order — only the breaks change, so the views' fixed orders are fine — but every
  photo may move the rows above it, so streaming replaced it with the greedy layout.
  It could still fit the full relayouts we already do (a view switch, a resize),
  with the greedy one appending. Code kept (`workers/layout/`); not planned.

## Later: the perceptor platform

Don't rush — a stable core first.

- Server → client events (New / Updated / Processed Item) as designed in
  [`Client flow.puml`](../puml/Client%20flow.puml) (MQTT): the client draws a
  preloader, fixes the grid, then the thumbnail. The delta itself is there
  (`/items?since=` + tombstones, PR #18): the client refreshes on its own moments
  today; a push would carry the same delta.
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

**Photos — renditions, not a transcode** (the asset contract already calls them
renditions) — libvips on the CPU; HEIC via libheif, RAW via its embedded preview (or
libraw). The win is not decoding faster but **not decoding a 12 MP HEIC at all**:
- **the source, cheapest first**: Photos' own JPEG when it is big enough for the size
  (`_1_102_o`, ~1536×2048 — JPEG is shrunk while it loads); the HEIC's embedded
  thumbnail for a tile; the full original only when nothing else will do. Most of an
  iCloud library has no local original anyway (3 528 of 5 866 photos here);
- **a decoder per type that can be swapped** (source → decode → transform → encode);
  libvips is the first and only one;
- **benchmarks on the real library** in this step: JPEG → 400 / 1600, HEIC → 400 /
  1600, on the Mac and on the i5 — images/s, CPU use, peak RSS (a slow one that
  parallelises well is fine). A macOS ImageIO decoder for HEIC is the next step only
  if HEIC → 1600 is the bottleneck (whether ImageIO decodes HEIC in hardware is to be
  measured, not assumed).
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
