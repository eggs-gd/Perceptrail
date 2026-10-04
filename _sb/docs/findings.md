# Findings & decisions

What practice taught: the decisions that are not obvious from the code, the
approaches already rejected (do not propose them again without a new reason), and
the traps. How things work now is in the module READMEs; this file keeps only the
why. Add new entries to their section, dated; when an entry becomes plain
architecture, move it into the module's README and drop it here.

Rewritten 2026-10-04 (PR #25) from the full log — the history is in git.

---

## Product

- **A viewer over a library, not a library** (2026-10-02). Immich, PhotoPrism,
  Lightroom, Apple Photos keep the library (sync, albums, tags, the heavy UX);
  Perceptrail is another way through the same photos. We read what they leave and
  never become a second copy of it — hence providers, and grouping that collects
  the whole asset package.
- **One endless sheet, no filters** (2026-10-01). Every perceptor is a slice of the
  whole library around the photo the user is at. A "videos only" button was not
  needed: the Length view puts every video first.
- **No release without the transcode** (2026-10-01): HEIC in Chrome, iPhone HEVC,
  big originals in the grid. The earlier bar ("one perceptor end to end") was met by
  geo and is not enough.
- **Item == asset** (2026-09-29): one entity, a whole group of files; a derivative is
  a file linked to its source (`LinkedTo`), never an item of its own.

## Gallery (svebapp)

**Rejected:**

- One worker message per photo: every message is its own event-loop task — a sort,
  a regroup and `animate:flip` measuring every tile per photo froze the tab ~90 s at
  3 000 photos.
- Batched worker messages: works, but against "the stream brings one photo at a
  time". The page subscribing to `layoutDb`'s visible window replaced messages.
- Clearing the view before a rebuild (every tile faded out and back); async layout
  tasks with cancellation (an aborted task unsubscribed the new one's hooks — photos
  gone for good); one `{#each}` per row (a tile cannot move between rows);
  debounce instead of throttle (a stream never pauses).
- Window / size subscriptions as `LiveQuery` in `$derived`: a new query per window
  step starts empty → tiles blink; they stay in `$effect` as the integration
  boundary.
- A fixed share of the side panel for the "no value" section (it flips the
  problem); linear by the sheet's height (two thirds for "no place"). Decided: √ of
  each section's photo count, at every level.
- The optimal (Dijkstra) layout for streaming: it does not reorder, but one photo
  may move every row above it. Code kept.
- `sessionStorage` for the per-tab layout database's name: a duplicated tab copies
  it.
- Polling and cutting the walk's pause after an on-demand request (a whole walk for
  one known photo); then `Regroup` injecting a group and waiting — replaced by a
  rework mark and the client's guess.

**Traps:**

- **A whole-table `liveQuery` re-runs on almost every transaction** (1 578 of 2 000
  in a per-item stream): O(N²). Subscribe to a bounded window; totals from a small
  record. Same trap through a record the window reads: `meta.layout` (relayouts
  only) and `meta.size` (per batch) are apart for that reason. Dexie itself
  propagates writes across workers fine (`BroadcastChannel` `storagemutated`; spike
  in `svebapp/spikes/livequery`).
- **A hidden or non-painting tab** (the agent's browser pane too): no
  `requestAnimationFrame`, `ResizeObserver`, scroll events. Measure animations in a
  visible tab; snapshots fall back to a 100 ms timer; check positions against
  `layoutDb`, not the DOM; `await import('/src/lib/workers/proxy.ts')` on the dev
  server gives the live module to drive by hand. Streaming issues need a slow
  backend (a mock with a delay).
- Scroll before the taller height is in the DOM is clamped: apply height and items,
  `tick()`, then scroll. `overflow-anchor: none`: the browser's own anchoring read
  as a user scroll and dropped ours.
- Re-taking the anchor on every resize walks the view back: take it once, keep it
  until the user scrolls. Tiles mounted by a relayout must not fade from 0 (a white
  flash when widening); a switch's relayout fades them by the wave.
- **Tiles stay `loading="lazy"`.** Blank tiles came from full-size originals in the
  grid (median 0.5 MB, 40 % HEIC): decoding a window of multi-megapixel images
  exceeds the browser's decoded-image budget. Dropping `lazy` made it much worse;
  since tiles show previews (Apple's derivatives, then our renditions) `lazy` is
  stable. Rule: never feed the sheet full-size photos — previews only.
- `?at` kept with a shallow `replaceState`: Back gives `page.url` without it — read
  it from `location`. Back restores the router's scroll, which looks like the user's
  and drops a switch's anchor.
- `LiveQuery.current` outside an effect or a template is `undefined` (the
  subscription is active only while something reactive reads it).
- The layout turns pointer events off for `<main>` under the viewer: anything fixed
  outside `.viewer` turns them on itself. `element.click()` does not hit-test.
- Dexie's `updating` hook gives changes by key path (`{"asset.original": …}`):
  apply them with `Dexie.setByKeyPath`, not a spread.
- A test that opens IndexedDB by hand and never closes it blocks every later
  version upgrade.
- **Plain HTTP on a LAN address is not a secure context**: no `navigator.locks`, no
  `crypto.randomUUID`. Syncs run unserialized there (the count check heals), pages
  answer a roll call on a `BroadcastChannel`. The agent's browser pane blocks a LAN
  page's requests to another port: proxy the API through Vite on the same origin.
- **Code at module initialisation must never throw**: Safari and Firefox print
  stacks as `fn@url:line:col` without a header; the logger's parser threw, and
  Safari's Kit export check then hit an uninitialised `load` — a 500 on every page.
- A global stylesheet nobody imports does nothing: `app.css` was never loaded until
  the root `+layout.svelte` imported it.
- Chrome answers `""` to `canPlayType('video/mp4; codecs="hvc1"')` and `probably`
  to `hvc1.1.6.L93.B0`: ask with a full codec string. MOV is offered as MP4 (Chrome
  plays it, does not claim `video/quicktime`).
- With `srcset` + `sizes=100vw` the browser sizes an `<img>` as 100vw whatever the
  file: the viewer's box is sized from the asset's biggest image.
- The browser cached on-demand answers for a day: the contract version is part of
  their URLs (`?v=`).
- SVG icons from plugins are drawn as a CSS `mask-image`: no script runs, the
  button's colour paints it.

## Sync and the server ↔ client contract

- **The cursor is the stream's last line**, not a header: once the 200 is out a DB
  error cannot become an error status. No last line = cut short = the old cursor
  stays; a line that cannot be stored fails the sync.
- **Several tabs share one IndexedDB**: one sync at a time (Web Locks), the last
  line carries `total` and a copy holding another count resyncs from nothing;
  changes reach every tab's layout worker over a `BroadcastChannel`; the layout
  database is per tab (width and view are the tab's).
- **The epoch is `<database>.<contract>`**: a change to what an item carries bumps
  the contract and every client takes everything once, in place (the sheet never
  empties — "critical", the owner); only another database starts from empty.
- **SQLite times are text with the writer's offset** (fractional parts of any
  length, some without a zone): compared as text they lie — `julianday()` in SQL,
  or compare in Go; deletions are asked a day earlier (a tombstone too many is
  harmless).
- Internal marks on items (`MarkRework`, `ClearHashes`) use `UpdateColumn`:
  `updated_at` stays, or the delta would stream the item in its old state.
- A refresh of the order must not run during a view switch (it aborts the switch's
  request).

## Perceptors

- **The server orders, the client lays out** (2026-10-01): perceptors are Go
  plugins, ordering in the client would mean writing each twice, embeddings do not
  fit a browser; the layout depends on the window width.
- **Sections ride in the order stream** (on the first item of a section, a path
  coarsest first), not a separate list: their positions come from the client's
  layout, and a relative perceptor's depend on the anchor.
- **Sections come from real values** ("12 MP", "3 min"), not fixed bands; the one
  fixed label is "no value".
- **Rejected: a perceptor opens a database of its own** — a cgo driver inside a
  `.so` (Go plugins need identical versions of everything), the data dir and
  credentials in every plugin, and the core blind to the data (deleted photos leave
  rows, a new version does not know what to recompute). **Rejected: an API for
  plugins to create tables** — the same, one step removed. **Rejected:
  `SetValue("lat", …)`** — untyped. Decided: the plugin declares a struct
  (`api.NewStore[T]`), the core keeps it.
- **A file per perceptor in SQLite**: one writer per file (an ML perceptor does not
  block the import), reset by deleting its file; no `ATTACH` (limit 10, per
  connection — awkward with a pool): values are read by a list of guids.
- **"Nothing found" is a row** (`has = 0`): else every walk sends an item without
  GPS through exiftool again. A new perceptor or schema takes one pass (items are
  marked for rework, not `Dirty` — that would hide them).
- `rows.Err()` after reading a store: an SQLite error mid-read only ends `Next()` —
  a partial map looked whole.
- **Geo is not one axis**: west → east puts Krakow next to Cape Town — a Hilbert
  curve; regions and cities ordered by their first point on it, so each is one span
  of the sheet. Sections from the place's time zone until there is a geocoder.

## Import (gontroller)

**Rejected on the way to the stages (PR #24):** the walker cycling itself (the
cycle is the service's); `Spread` steps and the walk's result handed to the gate
through a shared variable (a second input, a side channel); a `Pipe` carrying a
flush message with a writers counter (a hand-made `close` + `WaitGroup`); Refresh
injecting a group and waiting for it (a mark is enough); `Deps` / `Config` structs
for constructors (they hide coupling, not cut it); a `Steps` struct next to the
chain (a second declaration); exiftool passed in from outside identify; nine
one-function steps in identify; a 1 → N step for Apple assets (the grouper forms the
groups up front from the DB and a `stat`: each walked file closes at most one group).

**Deletions, two places** (2026-10-04, PR #26): the walk states a fact (a file
is missing on disk), the provider says what is gone for the library (`Asset.Missing`:
Apple's trashed assets are on disk; a file Optimize Storage offloaded may be kept),
the gate has the model apply it. The walk does not delete rows itself: "not on
disk" is not "not in the library". Rejected: `Prune` of the perceptors' rows every
pass (or "only when something was deleted") — they are reconciled once at start
(`exif.Reconcile`, with the rework of items a perceptor has no row for): a row left
behind is never read, and a soft-deleted item may come back in the same pass (a
move).

**Contracts per consumer** (PR #26, Codex review): the library contract
(`provider.Provider`) stays one; each consumer declares the part it uses
(`group.Library`, `route.Library`). Go does not convert `[]provider.Provider` to
`[]group.Library`, so `group.New` takes `[]L` for any `L` that is a `Library` (a
type parameter, no copying loop); `library.Of` reaches the routes through a
wrapper in `web` that returns a plain nil for a plain folder's item (a nil provider
inside the interface is not nil). Rejected first: two more producer-side interfaces
in `provider`. Likewise a lookup only the walk needs (`FindFile`) stays off the
model's broad `FilesApi`. An unused `Outcome` and `Name()` went instead of being
moved.

**Providers** (2026-10-02): rejected — a typed `sources:` list in the config (the user
would have to know what each folder is), marker files (`.immich`, `@eaDir`) as a
filter of their own, a claim step per provider. Decided: one switch, the first
provider that claims a file gets it, the plain folder last. A disabled provider is
not in the chain: its files are a plain folder's. A provider that comes later takes
its files over through the usual deletions (GUIDs change; carrying them over by
content is for when a second provider exists).

**Traps:**

- **`Changed` must be stored**, cleared only by validate: the walk writes the new
  stat at once, so a pass that fails before identify decides the group would lose
  the change.
- **The fingerprint is the file's bytes** (size + sha256 of the first and last
  64 KB), not the tags read: a change to the declared tags must not make every
  file new.
- **The walk writes only what changed** (2026-10-04): it used to read and write
  every file's row every pass (a `SELECT` + an `UPDATE` stamping `CheckTime`, a WAL
  commit each) — an idle pass over 10 000 files took 5.8 s, now 0.1 s. The rows are
  read once; what was not visited is missing (no stamp needed, `CheckTime` gone).
  A changed stat is saved as its columns only: the row read at the walk's start
  may have moved on (a role, sizes) by the time its page is written.
- **Photos' own files were 60 % of the files table** (2026-10-04, the owner's
  library: 28 253 of 47 609 rows — `database/search` 18 k, `resources/caches` 9 k),
  churning every pass for nothing: the Apple grouper dropped them. A library now
  names the directories it holds no media in (`Skips`) and the walk does not enter
  them — the walk itself knows no library. Rejected: the walk filtering by names
  of its own (a library's layout is its own business, as its claim is).
- **Deletions are dangerous**: a cancel, a missing root, an empty mount point or an
  unreadable directory (no Full Disk Access to the Photos library) would delete the
  library. Only after a complete walk that found files, never under an unreadable
  directory.
- Moves race with deletions: the validator restores a soft-deleted item with the
  same fingerprint (order-independent).
- **exiftool `-n`**: the composite `GPSLatitude` is signed by its Ref, the EXIF one
  is not (never ask it); `Rotation` is degrees in QuickTime but quarter turns in
  HEIC. `-j`, not `-s2` (no header for a single file). **No `-G`**: groups rename
  every tag.
- exiftool on broken files: garbage and empty files give `Error` ("File format
  error", "File is empty"); a JPEG cut after its header gives a type and no size; a
  text file named `.jpg` is `text/plain`. A group exiftool returns nothing for (a
  timeout) is retried — it may pass.
- **Dates**: `FileModifyDate` comes with a zone (most of a real library has no EXIF
  date); `0000:00:00` is no date; QuickTime `CreateDate` is UTC, EXIF's is local.
  Zone: the tag's offset → local − GPS time (rounded to 15 min) → the coordinates'
  IANA zone (`tzf`, embedded) → the server's; not longitude / 15 (no DST, wrong at
  borders). The offset is stored apart (`DateOffset`): SQLite and `timestamptz` give
  times back in UTC.
- An embedded RAW preview has no EXIF of its own: copy the RAW's Orientation onto
  it, or a portrait lies on its side.
- MIME detection never uses the system tables (a minimal Docker image has none).
- **SQLite**: WAL + `busy_timeout`, **one connection** ("database is locked"
  otherwise); never query through `db` inside a `tx` — deadlock.

## Apple Photos

Layout, the DB's facts and PhotoKit's behaviour: the
[library README](../../gontroller/internal/library/README.md#apple-photos).

- **One copy** (2026-10-02): a cache of ours beside the library adds a second copy.
  What Photos keeps is served as it is, what it lacks is asked of Photos and lands
  in its library under its storage policy. Our webp at 1600 would be ~4× smaller —
  a saving on a duplicate, so no reason.
- **On demand, never in bulk**: the tiles need no request (a ~360×480 master is
  local for nearly every asset); the medium on open plus ±6 neighbours; hover video
  on hover; only assets with nothing local at all are asked for in the background.
- **"Download Originals to this Mac"** is supported but not the only way: for decades
  of photos it forces the whole library onto the disk; asking per level is the mode
  Photos lacks.
- **PhotoKit from Go (cgo), not a Swift helper**: one process, cgo is there for the
  plugins anyway. Reading the library stays pure Go in every build (an archived
  library, a NAS copy); Docker on a Mac has no PhotoKit and no access to a
  `.photoslibrary`.
- **The Photos DB wins over the files' EXIF**: what the user sees and may have
  corrected; a cloud-only asset has nothing else.
- **The Original is the biggest of what the user sees** (Photos' current version,
  the edit, at full resolution): edits are the library's feature, not ours.
- **The tile's cloud means one thing for every kind**: the full resolution is not
  here.
- **HEVC in browsers (2026)**: Safari; Chrome / Edge 107+ on Windows and macOS;
  Firefox 134–137 by platform. Gaps: Chrome on Linux, no software decoder in Chrome
  or Firefox. So HEVC is the viewer's default, the 360p H.264 covers the gaps; no
  H.264 720p of our own.
- **The agent's shell cannot read `Photos Library.photoslibrary`** (macOS privacy):
  Apple files are 404 on a test server started from it; the user's server is the
  test, or a synthetic library (a copy of `Photos.sqlite` + sample files).

## Transcode

- **Software first, hardware as a step of its own**: the queue, sizes, outputs and
  HDR tone mapping are got right once on `libx264` (CI has no GPU); the software
  path stays the reference and the fallback.
- **Photos: CPU first, a hardware decoder only if measured**: JPEG through libvips
  shrinks while it loads; copying a 24 MP bitmap to the GPU and back can eat the
  gain; Immich renders photo previews on the CPU too. The bigger win is the source.
- **How others split it**: Immich — one image does API, previews and ffmpeg (workers
  by env), ML apart; PhotoPrism, Jellyfin — monoliths with ffmpeg as a subprocess.
  Common ground: previews in the core, ML apart.
- One codec and one image format in the config, not arrays (N× disk and time).
- **jellyfin-ffmpeg** in the image, not our own build: every hardware backend and
  HDR tone mapping.

## Go and the toolchain

- **Go plugins**: host and `.so` need the same toolchain and identical versions of
  every shared package (`perceplib`, zap, multierr, `x/sync`, testify…) — bump all
  modules together, then `make build-plugins`. `TestLoadExternalPlugins` builds and
  loads them; it must use the test binary's own Go (`GOTOOLCHAIN=` its `runtime.Version()`; `runtime.GOROOT` is deprecated), and run with `-count=1` (the test
  cache ignores files in other modules). Under `-race` the plugin must be built
  with `-race` too ("plugin was built with a different version of package
  internal/runtime/sys"): `test/pluginbuild` reads the test binary's own setting.
- External EXIF plugins once stalled the import: a channel was allocated for each
  but a step added only for the built-in ones — nobody read it. Allocate outputs
  only for real steps.
- gopls built with an older Go cannot read code for a newer one:
  `GOTOOLCHAIN=go1.27.1 go install golang.org/x/tools/gopls@latest`.
- **The gopls MCP saw nothing** ("could not import … in GOROOT", 2026-10-04): it
  runs from the repo root, which is no module, and with the system Go (1.26.3; only
  inside a module does `go` switch to the 1.27.1 of `go.mod`). Both were needed: the
  workspace `gopls.work` (via `GOWORK`) and the module's Go on `PATH`
  (`go env GOROOT` from `gontroller/`). Rejected: a `go.work` at the root — the go
  command would pick it up in CI and `make build-plugins`, unify the modules'
  dependency versions and hide the plugin ↔ host mismatch the plugin test exists
  to catch.
- **Layout by Go's own conventions** (2026-10-04): `internal/` (the one directory the
  toolchain enforces; `pkg/` came from `golang-standards/project-layout`, which the
  Go team does not endorse); `main` at the root until a second binary; `test/` is
  never linked into the binary. Package names singular, never a standard library
  name (`plugin` collided — now `perceptor`).
- **A module declares its own `Config` interface** over the one `*config.Config`
  (structural typing: no DTO per module). Rejected: per-module config structs built
  in `main`, an `AppContext` singleton of globals, `library.Use` (a setter only
  tests called).

## exiftool

- The wrapper is our fork, `github.com/eggs-gd/go-exiftool` (upstream unmaintained).
  A clone with its own `go.mod` inside gontroller became a separate module — hence a
  tagged package. `gh pr create` in a fork targets upstream: always
  `--repo eggs-gd/go-exiftool --base main`.
- **The distribution**: exiftool.org no longer serves tarballs; SourceForge keeps
  only the last few versions, CPAN keeps production releases forever. Not every
  version on the site is production — use the latest production release. A
  `dist-<version>` tag builds, tests and publishes it (`dist-*` is not semver: no
  clash with module tags); the Docker image pulls exactly that.
- v0.5.1: a per-command timeout with restart, `Wait` after `Kill` (no zombies),
  stdout kept with a stderr error.
- **An orphaned ExifTool runs forever** (2026-10-04, ~200 perl processes on the dev
  box): with `-stay_open` it never exits at the end of its argfile — `ReadStayOpen`
  polls stdin (`sysread`, then `select 0.01`) forever, so every server process that
  died without `Close` (SIGKILL from an IDE, a crash, the 10 s shutdown limit, a
  killed `go test`) left its whole pool. Fixed in the fork: next to every ExifTool a
  watchdog shell blocks on a pipe only the Go process writes; when the process dies
  the pipe closes and the watchdog kills ExifTool at once. Both are the Go
  process's children, reaped by it; the watchdog is stopped as soon as ExifTool is
  reaped. Rejected on the way (Codex review): ExifTool under the shell (start
  errors turn asynchronous; the watchdog, ExifTool's child, became a zombie under a
  PID-1 Go process and could signal a reused pid after polling); `Pdeathsig` (Linux
  only, and it fires when the spawning *thread* exits — Go's threads come and go).
- The pool never panics: an exiftool that cannot start fails the pass's commands
  with its reason; the next pass tries again.

## Repository

- **perceplib is a subtree, not a submodule**: detached HEADs, gontroller depending
  on uncommitted perceplib changes, two commits per change. It stays a separate
  public repo with tags for third-party perceptors; publishing is explicit
  (`perceplib/README.md`). A monorepo module later, once Perceptrail is public.
- **One fact, one place** (2026-10-04, PR #26): a contract change (the walk's message)
  touched 41 files, ten of them docs telling the same fact five times (two READMEs,
  the program's README, two diagrams with type names). The rule is in AGENTS.md
  ("Documentation contract"): the owner writes it, the rest link; diagrams name
  steps, not types; no lists of test files.
- Releases are a fast-forward of `master` to a tagged `develop` commit, never the
  GitHub rebase button (it rewrote every commit in PR #2 and the branches diverged).
- `npm`: a stale lock pinning an old plugin — regenerate the lock, no `--force`.
  `cookie@0.6.0` comes from Kit itself; the audit's "fix" downgrades Kit — left.
  TypeScript 7 is not supported by Kit and svelte-check yet.
- ImageMagick without rsvg drops SVG strokes: rasterise with `resvg`.
