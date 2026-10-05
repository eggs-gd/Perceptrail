# Roadmap

Status as of 2026-10-04 (PR #25). What exists is described where it lives — the
module READMEs ([gontroller](../../gontroller/readme.md),
[importer](../../gontroller/internal/importer/README.md),
[library](../../gontroller/internal/library/README.md),
[perceptors](../../perceptors/readme.md), [svebapp](../../svebapp/README.md),
[go-chain](https://github.com/eggs-gd/go-chain)); why it is so and what was
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
  narrowed by its consumers · #27 exiftool: no panic, no orphans (go-exiftool
  v0.5.2); the plugin tests under `-race` · #28 the gopls MCP over every Go module
  (`gopls.work`) · #29 the walk in pages, no write for an unchanged file; a
  library's own directories (Photos' database, caches) not walked. The PR #24
  review is done · #31 the chain and the logger as libraries
  ([go-chain](https://github.com/eggs-gd/go-chain),
  [go-zap-decor](https://github.com/eggs-gd/go-zap-decor)) · #30 the write bus:
  the model's one writer and a read pool on
  [go-pub-sub](https://github.com/eggs-gd/go-pub-sub), every write rule a topic
  (sync and asynchronous), `query` / `tx` / `Proxy`; the designs of the bus and the
  work queue.

## Releases

- **0.2.0 — the first release**: the transcode (photos and video, hardware video
  encoding) and Docker. A release is an image someone installs; without the
  transcode HEIC does not show in Chrome, iPhone HEVC does not play, big originals
  slow the grid. Until then everything stays in `develop`.

## Next: the expensive stage (transcode)

In steps, each its own PR (designs below: "The write bus", "The work queue",
"Expensive stage"):

0. [ ] **The write bus** — [go-pub-sub](https://github.com/eggs-gd/go-pub-sub) (operations as topics, subscriptions,
   classes) and the model's writer. Done: one write connection, a read pool,
   batches of what is queued, lanes by class, `synchronous=NORMAL`; every write
   rule a topic — the public method waits for its result, `…Topic()` gives it
   asynchronously (nobody uses that yet). Next: the classes' deadlines and the
   callers going asynchronous (the walk without its own pages, the steps each its
   own way). Before any new writer comes.
1. [ ] **The work queue and the render service** — the `work` and `renditions`
   tables, `render.New(cfg, db, logger)` with `feed → source → render → commit` and
   a stand-in renderer (a copy): the queue's rules tested before any codec.
2. [ ] **Photo renditions** — libvips on the CPU, the source chosen to avoid a full
   decode (Photos' JPEG, the HEIC's embedded thumbnail, a RAW's embedded JPEG),
   benchmarks on the real library.
3. [ ] **Video, software** — `libx264`, HDR → SDR, the hover clip, the poster; the
   codec → encoder table and the probe with a software fallback from day one.
4. [ ] **Docker** — the image (jellyfin-ffmpeg), a base compose with software
   encoding; the docs say a Mac with Photos runs the native binary.
5. [ ] **Hardware** — QSV (`hwaccel.qsv.yml`), VideoToolbox (native on a Mac),
   NVENC when there is one to test on; a macOS ImageIO HEIC decoder only if the
   benchmarks show HEIC is the bottleneck.

0.2.0 needs 0–4; 5 is wanted. Only the plain folder's items are rendered (a library
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
the top knows no tool. **Every chain is a service of its own** in `main`
(`render.New(cfg, db, logger)`, …), shaped as the importer: `Start` runs pass after
pass, a pass is a new chain that ends from its input (its `feed` returns when the
queue is empty), then a pause (polling its queue). Services know nothing of each
other: what one leaves in the database is what another's queue finds. Apart, not as
stages of the import: a video that takes minutes must not hold the import's pass;
the resources differ (IO and exiftool, CPU / GPU, ML); roles later
(`roles: [render]` on a GPU box) are a service switched on or off.

- **Render** (the expensive pass): `feed → source → render → commit`. Feed: a page
  of the `work` queue (the plain folder's items lacking current renditions),
  leased. Source: what to decode, cheapest first (a big enough JPEG of the group,
  an embedded preview, the original last; exiftool lives here). Render: a switch by
  kind (photo: libvips; video: ffmpeg; Live Photo), hardware inside. Commit
  (class Now): discard if the item was deleted or its fingerprint changed
  meanwhile, else the renditions, `Ready`, the queue's row done.
- **Pixel perceptors (ML)**: `feed → pixels → ml → commit`. Pixels from the render
  chain or the provider (Photos would mean asking in bulk — local thumbnails may do
  for faces). Maybe a service of its own (goMLer).
- **Group perceptors** (journeys, face clusters, series, duplicates): `feed → group
  → commit`, over what changed.
- **Maintenance**: `find → prune` — renditions of deleted items, perceptor values,
  orphans.
- **API providers**: see Providers.

## Smaller open items

- [ ] **Booleans set by hand** (AGENTS.md "Code Style"), each where it belongs:
      - small, with the next change of their code: `perceptor.loaded` → `sync.Once` or the registry itself;
        `apple.asset.sent` → a sent group leaves the map; `walk.Result.Complete` →
        `Result.Err` (why it is incomplete), `Complete()` = no error;
      - `dto.FileDto.Changed` (stored) → the stat the group was validated with,
        `Changed()` = the stat differs from it — a PR of its own (a schema change,
        a migration);
      - `dto.ItemDto.Rework` (stored) goes with the `work` queue: a need derived from
        versions and inputs, not a mark;
      - `dto.WalkedFile.Missing` stays for now (a fact in a message).

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
- [ ] **Plugins anyone can build: WebAssembly instead of Go plugins** (idea,
      2026-10-05). A Go `.so` is a piece of the same binary — the same toolchain,
      the very same versions of every shared package, cgo, Linux / macOS, no
      unloading: someone else's build does not load. The contract moves from Go
      types in shared memory to a **protocol** (serialised data, a protocol
      version — not package versions):
      - **a perceptor is a `.wasm` file**, one for every OS and CPU, built from any
        language that targets wasm (Go `go:wasmexport` / TinyGo, Rust,
        AssemblyScript, C); the host runs it **in its own process** with
        [wazero](https://wazero.io) (pure Go, no cgo), maybe
        [Extism](https://extism.org) for the plugin side. A call is a function
        call plus copying bytes in and out of the module's own memory
        (microseconds), not a request; a sandbox — files, network, the database
        only through functions the host gives; a crash is an error, not the server
        down; a pool of instances for parallel work;
      - **perceplib becomes an SDK and the protocol's description**, no longer
        linked into the host: no version matching;
      - **most perceptors declarative**: they declare their value, order and
        sections (as `OrderByValue`), the host orders; only one cheap call per
        item crosses the boundary (tags → value); code for `Order` only where it
        is special (the Hilbert curve, the trail), streamed in pages;
      - **goMLer** hosts the pixel perceptors the same way: the plugin
        pre/post-processes, inference (ONNX Runtime / GPU) is a host function
        (`load_model`, `infer` — as WASI-NN, which wazero lacks); Python ML, if
        ever, as an external worker over gRPC or the bus. gontroller ↔ goMLer
        talk as services (gRPC / MQTT), not per plugin;
      - steps: a spike (`exif_geo` as wasm under wazero, the cost of a call per
        item, the size), protocol v1 (`Describe` → `Process` → `Order` where
        declared), then the `.so` loader goes; built-in perceptors stay built in.
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

## Design: the write bus — what is left

Built: one writer, a read pool, every write rule a topic — the
[model README](../../gontroller/internal/model/README.md). Why, measured:
findings "The write bus". Open:

- **The classes' deadlines** — how long a rule tolerates waiting for a batch, a
  property of the rule:

  | class | waits | for |
  |---|---|---|
  | **Now** | never; its own lane, taken first | render's commit, on-demand marks, the web |
  | **Frame** | ~33 ms or ~1000 operations | the import's steps, the walk, perceptor values |
  | **Idle** | ~250 ms or ~5000, only while nothing else waits | `Reconcile` / prune, maintenance |

  Only for callers that are asynchronous: one waiting for each result in a loop
  would pay the deadline per call. A transaction cut at ~20–50 ms of work, so Now
  never waits longer.
- **The callers asynchronous**, each its own way (`chain` and everything above the
  step unchanged) — on the command layer's clients ("Design: events and commands"):
  the walk submits and sends a file on when its result came — its pages go; a step
  that needs the answer for its output (validate: the item a group became) waits
  inside, or keeps its own queue.
- **Perceptors never see the bus**: the core methods a perceptor may call become
  interfaces in `perceplib/api`, declared by their consumer — synchronous, or
  asynchronous with a subscription; the core implements them. Added with the first
  method a perceptor needs (none yet: values go through `Store.Put` in the chain).
- Later, from qwr's ideas: error classes (lock / constraint / schema) and a
  dead-letter list for operations that failed for good.

## Design: events and commands (go-pub-sub, the next version)

go-pub-sub's `Op` today is a command and a broadcast at once: a subscriber gets
everyone's results and picks its own by ID. Laid out in layers instead — the
library stays a mechanism, how to handle an event is the product's decision (its
patterns go to the library's docs and examples, not its API):

- **Event** — `Topic[E]`: `Publish(e)` and `Subscribe(fn func(E)) *Sub`, `Close`.
  For the publisher a black hole: nobody subscribed — nothing happens; subscribed —
  each one's `fn` is called, in order, right there (synchronous dispatch, as in UI
  engines). The contract: a listener is thin — it hands work off (to its channel,
  its queue, its workers) and returns. The bus recovers a listener's panic and logs
  one that took too long, so a broken contract is seen, not guessed. No channels, no
  queues, no capacities in the API: what a listener does with an event is its own.
- **Command** — `Op[T, A, R]` run by an executor (the model's writer): `Do(a)`
  waits for its own result; a `Client` gets **only its own** results — no foreign
  traffic, so nothing of its own is lost — and counts its room when it submits
  (`Submit` waits while its results are not read). The shapes stay (`Message`,
  `Signal`, `Trigger`).
- **The bridge** — a command that finished is an event: `op.Done()` is a
  `Topic[Result[R]]` anyone may listen to without submitting.

**Domain events, after the commit.** What others react to is not a rule's raw
result but a fact of the library: `ItemPublished{GUID, State}`, `ItemGone{GUID}`. A
rule emits it inside its transaction (`tx.emit`); the writer publishes it only
after the commit, and a rule rolled back to its savepoint takes its events with it
(an outbox in memory — no one hears of a write that is not there). Across processes
later (goMLer on another box): the same `Topic` over another transport (NATS,
MQTT — `ML Flow.puml`), or a persistent outbox if an event must outlive a restart.

**The database is the truth, an event is a hint** (level-triggered): an event says
"look", the listener reads the current state and acts on it; a missed, doubled or
late event changes nothing — the polling stays as the safety net, the event only
cuts the wait. Each consumer handles it its own way:

- **the walk** — a transit: submits through its clients, waits for its results,
  sends a file on; its buffer is the results not yet back — its own business. When
  it waits on the first, the rest wait too and go together once the batch commits:
  that is the batching;
- **render / transcode** — bounded on purpose: an event only wakes it; its pace is
  its workers (CPU cores, GPU slots), each taking its next item from `work`;
- **the web** — later, a push to the clients on `ItemPublished` instead of their
  polling `/items`.

Steps: go-pub-sub — the layers (`Topic`, `Client`, `Done`), docs with the handling
patterns (a wake-up, a keyed queue, workers) as examples; then the model's domain
events (`tx.emit`); then the walk on clients; render wakes on `ItemPublished`.

## Design: the work queue

SQLite first (decided): the design is tuned to what one file does well — short
transactions, indexed queries, one writer. Postgres stays the way up, not a
rewrite: what works on SQLite only gets better there (proper concurrent writers,
richer indexes, real transactions across connections); the bus does not know
there is one writer — that is the SQLite executor's business, a Postgres one may
write over several connections.

- **In the main database: the queue and the renditions** (the core's facts); a
  perceptor's results stay in its own file.
- **What is needed is derived, not recorded**: a query — "the item is alive and has
  no current result for this slug". A new photo enters by itself, a deleted one
  leaves by the join, a changed config or a new perceptor version makes the need for
  everything; nobody has to remember to enqueue (as the perceptors' rows and
  `Reconcile` work today).
- **One row per item and slug** — `work(guid, slug, …)`: a slug is the render or a
  perceptor; slugs are independent (faces, objects, colour run in any order, each at
  its pace), not stages of a chain. A new perceptor is new rows, not a new schema.

  ```sql
  CREATE TABLE work (
    guid        TEXT NOT NULL,     -- the item (= asset)
    slug        TEXT NOT NULL,     -- "render", "faces", "color"…
    version     INTEGER,           -- done with: the render config's hash / the perceptor's version
    input       TEXT,              -- done for: the item's fingerprint
    step        TEXT,              -- the perceptor's own stage (detect → embed…); the core never reads it
    done_at     INTEGER,           -- unix seconds: SQLite keeps times as text, text compares lie
    attempts    INTEGER DEFAULT 0,
    next_try    INTEGER,           -- backoff after failures
    error       TEXT,
    lease_until INTEGER,           -- taken into work until then
    PRIMARY KEY (guid, slug)
  ) WITHOUT ROWID;

  CREATE TABLE renditions (guid, version, size, format, w, h, bytes, path);  -- cache/r/<guid>/<version>-<size>.<format>
  ```
- **The next page** of a slug walks the items' `(state, date)` index — Waiting first,
  then Visible newest first, a keyset by date — and looks each up by the primary
  key: 1–2 ms for 64 at 200 k items, even with everything due. Not
  `ORDER BY (state = Waiting)`: it sorts everything (95 ms).
- **A lease** (`lease_until` = now + 15 min) when feed takes an item; commit clears
  it; a crash lets it expire and the item is taken again.
- **Failures back off**: `attempts++`, `next_try` 1 min → 10 min → 1 h → 1 day;
  after 5 the item waits for a new `input` or `version`; the error is kept (the info
  panel can show it). Without it a broken video would come back every pass.
- **Changed meanwhile**: commit compares the `input` taken with the item's now — a
  new fingerprint discards the result, the item stays due.
- **`Rework` goes**: today a stored mark (set by `MarkRework`, cleared by
  `Publish`) — a queue kept as a flag; with `work` the need is derived (a missing
  row, another `version` or `input`).
- **A config change** (sizes, format, codec) is a new `version`: re-rendering is
  lazy, maintenance prunes the old files.
- **A slug may require another**: a pixel perceptor's query asks for `render` done
  with the same `input` (it works on our pixels) — a condition, not an order.
- **Group perceptors** (journeys, face clusters) are not per item: a watermark row
  per slug ("changed since I last ran"), in the same table with `guid = ''`.
- **The item's state**: render's commit makes it `Ready` (`updated_at` moves: the
  client's delta brings it, Waiting photos finally show); the asset contract gets
  the renditions as files of their own role with `w`, `h`, format (`srcset`), a new
  contract version.
- **Idle polling**: when nothing is due the query scans every item (143 ms at
  200 k per slug). A full scan at start and after a version change; between them
  only the items changed since the last pass (`updated_at` is indexed).

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

- **The queue**: "The work queue" below — what is needed is derived, only what was
  done, failed or taken is recorded. A move changes nothing (outputs live under
  the GUID: `cache/r/<guid>/…`); long work checks between stages and is cancelled
  with its context (`exec.CommandContext`).
- Perceptors run on both passes (incremental refinement): a pixel perceptor's row
  in `work` records the input it ran on.
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
