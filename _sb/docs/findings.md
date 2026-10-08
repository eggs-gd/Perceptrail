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
  **The missing rows are read again before they are sent** (Codex, PR #29): the
  rows are from the walk's start and the chain works meanwhile — a moved file
  validated before the walk ends has its old row deleted and its item moved; sent
  from the snapshot, that row went to `Gone` with its stale `LinkedTo` and deleted
  the moved item.
  A changed stat is saved as its columns only: the row read at the walk's start
  may have moved on (a role, sizes) by the time its page is written.
  The walk sends a page only once its rows are written (a new row's `ID` comes from
  the database, and the steps after it update rows by `ID`): the chain gets files
  in pages of 256, not one by one. Kept on purpose (the owner): the client's
  progress comes from the server's answers, not from how the walk finds files,
  and next to the expensive stages the difference is noise. Rejected: holding back
  only new files (a stream while nothing is new) — order-preserving bookkeeping
  for no visible gain.
- **Photos' own files were 60 % of the files table** (2026-10-04, the owner's
  library: 28 253 of 47 609 rows — `database/search` 18 k, `resources/caches` 9 k),
  churning every pass for nothing: the Apple grouper dropped them. A library now
  lists the directories under the root it holds no media in (`Skipped(root)`), the
  import collects them every pass and gives the walk the list — data, as the
  perceptors' tags go to identify; the walk knows no library. Rejected: the walk
  filtering by names of its own (a library's layout is its business, as its claim
  is); a predicate passed into the walk (a lambda across modules). The root is an
  argument: the library's own root (its config at `Enable`) need not be the one
  walked.
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
- **SQLite**: WAL + `busy_timeout`; one writer (see "The write bus") — no more
  "database is locked", and reads no longer queue behind writes. A write that calls
  another public write runs it in its own transaction (the writer would wait for
  itself). Close the read-only connections first: the last connection to close
  merges and removes the journal, and a read-only one cannot (smoke caught `-wal`
  and `-shm` left behind).

## The write bus and the queue (SQLite, 2026-10-04, design)

Design: roadmap "The write bus", "The work queue". SQLite first (the owner): tuned
to it, Postgres the way up — what works here gets better there.

**Measured** (SQLite 3.51, WAL, this Mac; 200 k items, 1.08 M queue rows — six slugs
an item):

| | |
|---|---|
| 1.08 M rows in one transaction / 100 k | 1.1 s / 0.28 s (~1 M rows/s) |
| a point read by primary key | microseconds |
| the next 64 of a slug through the `(state, date)` index | 1–2 ms, even with everything due |
| the same with `ORDER BY (state = Waiting) DESC` | 95 ms: sorts everything |
| nothing due: a full scan to know it | 143 ms at 200 k |
| 1000 one-row commits: `synchronous=FULL` / `NORMAL` | ~0.17 / ~0.03 ms a commit |
| the database | 83 MB |

- **What degrades SQLite is not size** (281 TB, 2⁶⁴ rows; ours is hundreds of MB to
  a few GB) but one writer at a time, queries off an index (~0.7 µs a row scanned)
  and a WAL kept from its checkpoint by long readers; `VACUUM` after mass deletes.
  **Our real limit was ours**: one connection for everything (`MaxOpenConns(1)`
  against "database is locked") — reads wait behind writes. One writer + a read
  pool is the fix.
- **Grouping commits gains little in speed** here (a commit is ~0.17 ms with FULL);
  the bus is for one writer whatever writes, parallel reads, and steps that never
  stall on a write.
- **The writer in place, callers still synchronous** (2026-10-05): the import of
  10 000 files — first pass 4.0 s (4.6 s before), idle passes 0.09 s (0.11–0.13 s);
  the grouping comes from callers writing at once (identify's five readers). No
  deadlines yet: a caller waiting for each result would pay one per call.
- **A write and its two faces** (the owner, after two wrong turns): a write is
  the unexported method (its debugged logic, untouched); the public method of its
  name submits and waits (`Do`), its `…Command()` submits and goes on. Rejected: the
  public methods as mere pass-throughs to their twins with nothing to show for it
  (and a write calling a public write that came back to the writer); folding the
  bodies into closures inside the public methods (the working methods taken apart,
  to be taken apart again for the asynchronous side). Then one type playing two roles: the
  `Proxy` was the model and, with an `inRule` flag, a write's transaction — a public
  write called from a write was caught at run time (a panic) where the types should
  not offer it. Now three types: `query` (the reads, over the pool or a
  transaction), `tx` (a write's: the reads and the writes, no public ones), `Proxy`
  (the reads over the pool, the public writes, their commands). Every write takes one
  argument and gives one result (`pubsub.None` where there is none), so its method
  on `tx` is its command's function as it is; go-pub-sub's shapes (`Message`,
  `Signal`, `Trigger`) keep `None` out of a caller's hands.
- **A write, not a rule** (the owner): the logic on `tx` was called a rule, and the
  code read as policies; it is just logic that runs — a write (the method on `tx`)
  submitted as a command (the `Op`). "Rules" stay only for the library's own
  (when a group needs work, what a file gone means). The commands sit in the
  `Proxy` (embedded): an `Op` holds its executor, the model's writer — package-level
  ones would make the model one per process.

- **Who may write what is decided by who holds what** (the owner asked: a `Job`
  can run anything — "remove all tables"): `Job` is sealed (only an `Op` makes
  one), an `Op` needs the executor and the model keeps it to itself, so every write
  is the model's; steps and perceptors get `Command`s — an argument for a given write,
  no transaction, no code. In one process that is discipline, not security: any
  code in it can open the database file; today's `.so` plugins could delete it.
  Only isolation (WebAssembly, a process) makes a plugin unable to — and what it
  is given is the core's methods as `perceplib/api` declares them.

**Rejected on the way:**

- One batching for everyone: a writer that does not care (the walk) and one whose
  every result matters (render's commit) want opposite things — classes, by how
  long a write tolerates waiting.
- A batch timer (30 fps) for synchronous callers: one waiting in a loop pays the
  deadline per call (1000 validates × 33 ms). Kept as a class (Frame) once nobody
  waits synchronously.
- A result channel per call; then `any` results in one stream per caller — a command
  per write (`Op[A, R]`) is typed and needs no routing.
- Blocking or failing a subscriber that does not read: the writer never waits —
  a lost subscription is its owner's loss. (Superseded by go-pub-sub v0.3.0: a
  `Client` counts its room at submit, so delivery never waits and nothing of its
  own is lost — "Events and commands" below.)
- A new primitive in `chain` for asynchronous steps: a step keeps its own
  operations; `chain` and everything above stay as they are.
- A write split into a read outside and a write through the bus: decisions on stale
  data. The whole write is one operation in the writer.
- Ready-made packages: [qwr](https://pkg.go.dev/github.com/jpl-au/qwr) (a SQLite
  writer with job IDs — but its batches are not one transaction, it takes SQL
  strings, a default driver not ours, 25 dependencies, pre-1.0);
  [go-relay](https://pkg.go.dev/github.com/binozo/go-relay), kelindar/event and the
  like (broadcast only: no submit → result by ID, no batching) — the bus is a few
  dozen lines on channels.
- The bus in perceplib (first written there): it is the core's mechanism and its
  restrictions; a perceptor needs only the list of core methods it may call —
  interfaces in `perceplib/api`, synchronous or asynchronous, whatever implements
  them. Then a library of its own:
  [go-pub-sub](https://github.com/eggs-gd/go-pub-sub) — the restrictions stay the
  model's (it keeps the executor).
- The queue as a column per perceptor (a schema that changes with the loaded `.so`
  files, a dead column per removed one, an index per column, five columns of state
  each); the queue in each perceptor's database (every poll a join with `items` in
  another file — no `ATTACH`, so guid sets diffed in Go, O(N) per poll; leases and
  backoff written into foreign files); rows of "to do" (someone must remember to
  enqueue; a config change or a new version would not enqueue anything).

## Events and commands (2026-10-05, design)

Design: roadmap "Design: events and commands". The owner's line: the library is a
mechanism; how to handle an event is the product's, per consumer (the walk a
transit, render bounded by its workers) — patterns go to docs and examples.

**Rejected on the way:**

- The `Op` as command and broadcast at once: a caller's results mixed with everyone
  else's in its subscription, picked by ID — and lost when foreign traffic filled
  its buffer. A command's results go to its caller (a client), a broadcast is an
  event.
- Delivery "policies" (drop / latest / block) as a library enum: a capacity of the
  subscriber's channel dressed up as an API.
- A channel in the subscription API (`Subscribe(chan<- E)`): how a listener handles
  an event is its own — a channel is one way of many.
- A queue and a goroutine per subscription inside the library, so the publisher
  never waits: it hides the backlog. Synchronous dispatch with thin listeners — the
  rule of every UI engine — and the bus catching a panic and logging a slow listener.
- Kubernetes-style keyed work queues in the library: needed there because strangers
  write the handlers of a public product; here an example in the docs.

## The expensive stage: the queue beside the chain (2026-10-07, #36)

- **The chain stays, the queue is added** (the owner, after an ECS design): the
  chain is how one piece of work goes through its steps, the queue which items still
  need which work — different questions. The cheap stage is one pass with one
  exiftool read per group; split into systems, each would read it again. ECS stays
  a lens for where work belongs (roadmap). Rejected: the chain ending at identity
  with a scheduler of systems for the rest.
- **Events from an outbox per savepoint**: the writer's env became a batch (its
  connection and the events its writes emitted); a write's events are cut back when
  it rolls back, the batch's sent only after the commit. An event out of a rolled
  back write would wake render for nothing — harmless (it reads the DB), but a lie.
- **The queue's messages are `dto`**: render names `dto.Cursor`, `dto.WorkDone`,
  `dto.WorkFailed`, `dto.ItemPublished` and its own `Store` — never `model`, as the
  import's steps since #26.
- **A library's items are finished with no renditions** (Apple renders itself):
  filtered in Go (`library.Of`, no SQL for it), they would be due on every pass;
  finished as "nothing to render" for this version they leave the queue.
- **The stand-in is a symbolic link to the original**: a hard link changes the
  original's link count and ctime — a write into a Photos library, which a disabled
  Apple provider leaves to the plain folder (review: Codex); a copy doubles the
  disk. Render stays off by default while it is a stand-in.
- **A result carries its lease's token** (review: Codex): a worker that ran past its
  lease could overwrite the result of the one that took the item next, or a late
  failure replace a success.
- **Proved by breaking it**: the queue's tests fail when the lease or the input
  condition goes, the events' test when a rolled back write keeps its events;
  render's integration test reads the work row (`Work`) — "not due" alone is also
  what a lease looks like.
- **The stand-in alone had no product value** (the owner): the queue, the events and
  a renderer that shows nothing are plumbing; the PR went on to photo renditions —
  one feature, its steps as commits (AGENTS.md), and the real renderer tested the
  queue's design before it merged. The owner's measure: new code is added, old code
  is not rewritten; where old code had to change, the cut was missing.
- **`vipsthumbnail` in a process, not a cgo binding** (bimg, govips): a broken file
  fails a child process, not the server; a timeout kills it (`CommandContext`); the
  build gains no cgo. A process per image costs milliseconds next to the decode. It
  shrinks while it loads where the format allows (JPEG). Never upscaled: the sizes
  go smallest first and stop once the original is smaller (a srcset with two equal
  widths is invalid). Metadata stripped: a rendition is pixels, no GPS.
- **`publish` keeps an item Ready while its renditions are for its fingerprint**: a
  second pass of the cheap stage (a perceptor's rework, Photos made a file local)
  used to set Waiting / Visible from the preview — the item would vanish, and render
  would never come back (its work is done for that input). `NeedsWork` counts such
  an item through the cheap stage too, or every walk would send it again.
- **The cut the renditions showed**: the stream of items was `fn(item, files)`; a
  third fact meant a new parameter in every caller. Now `dto.StoredItem` — a new
  fact is a field. The renditions come per page like the files (one query each), not
  one query per item (50 k items, 50 k queries).
- **Smoke on the owner's library** (2026-10-08, a copy of the DB, Mac M-series, 4
  workers): 7 024 items in the queue, done in about a minute; 571 plain-folder
  images rendered in 47 s (about 12 a second) — 531 PNG screenshots, 36 JPEG, a
  HEIC, an AVIF; no failure; 85 KB an item on average (10 KB at 400, 75 KB at 1600:
  screenshots — photos will weigh more). Apple's items were finished with nothing to
  render; the 8 left Waiting are videos (no video renderer yet).
- **The cache is a tree by the GUID** (the owner: thousands of GUID directories on
  one level): `<part>/<ab>/<cd>/<guid>/` (`internal/cache`), as git's objects and
  Immich's thumbnails — renditions and extracted previews alike. Two levels of two
  hex characters: at most 256 entries a level, ~15 items a leaf at a million. A
  directory per item stays (its sizes); previews already extracted keep their
  stored paths.
- **A picture is a picture: no versions in the files** (the owner, after a version
  per libvips and a directory per version): what matters is the size, the format
  and the quality on disk, not what made them. A rendition is a size in a format,
  `r/<ab>/<cd>/<guid>/400.webp`, and a new render replaces them all; the queue's
  version is what they are (`400_1600-webp-q80`) — another config re-renders, a new
  libvips does not. Failures are not stopped for good either: after 1 min, 10 min,
  1 h, once a day — a broken file costs a failed try a day, and one a newer libvips
  reads comes back by itself (what the libvips version was for). The renditions an
  earlier layout kept per version are dropped once by `Open`, and rendered again.
- **A missing tool is not an item's failure** (review: Codex): without vipsthumbnail
  every item failed and stayed stopped — fixing the path changed nothing. Render
  checks `vipsthumbnail --vips-version` at start and does not run without it.
- **One rendition per size, no two of one width** (review: Codex): the config sorts
  the sizes and drops repeats (two of one size clashed in the database and the item
  never finished); an original exactly as big as a size stops there — Codex's `<=`
  on the requested size would also have stopped a 4000 px original shrunk to exactly
  400, so the check is "no bigger than the rendition before".
- **Render sweeps its cache by the database** (the owner: no deleting thousands of
  directories by hand): the model prunes the rows no one shows (`Prune`), render
  removes every file under `r/` the database no longer lists, then the empty
  directories — at start and hourly. Mark and sweep, not "delete version X": it also
  takes what a crash or an old layout left. A file or directory younger than an hour
  stays (a worker writes before Finish lists). Trap: removing a file touches its
  directory's time — a directory the sweep emptied goes at once, or the grace would
  keep every one (the test caught it).
- **Videos: one rendition for the tile and the viewer** (#37): the client plays an
  asset's `motion` on a tile's hover and in the viewer; `onDemand.hover` only when no
  video is here (Apple, on demand). So our 720p H.264 in `motion` is both — no hover
  clip. Rejected: our clip in `onDemand.hover` (the owner's first choice) — an
  `onDemand` means "ask the server for the medium" to the client: the viewer would
  prefetch an empty medium URL for Live Photos and show on-demand tools for videos.
- **ffmpeg's colour, three traps**: the encoder takes the colour tags from the
  frames, not from `-color_trc` (it wrote nothing) — `setparams` at the end of the
  chain; `zscale` finds "no path between colorspaces" from untagged input — the
  input's transfer, primaries and matrix (ffprobe) are given to it; and SDR tags
  only after a real tone mapping (review: Codex) — without `zscale` the HDR samples
  keep their own HLG/PQ tags and the browser maps them; HDR tagged BT.709 shows the
  wrong brightness.
- **A list in the contract is never null** (#37, the owner saw a black viewer):
  prepending our motion with `append(ours, a.Motion...)` gave nil when both were
  empty — `"motion": null`, and the client's `[...asset.motion]` threw on every
  item. The route test now checks every list of the asset; the contract went to 7,
  so a client that stored the nulls syncs from nothing (a delta would never bring
  the unchanged items back).
- **A long encode renews its lease** (review: Codex): a video's time is three times
  its length, a lease 15 minutes — past it the safety pass took the item again, the
  first result was dropped, and a long video could never be accepted. While an item
  renders its lease is renewed every 5 minutes, by its token; a lease lost stops the
  work. Two more holes (review: Codex): a page of 64 was leased at once while
  N workers took it one by one — its tail's leases ran out in the queue; now an item
  is taken only when a worker is free for it. And ffprobe ran without a limit (the
  encode's own is derived from its answer) — a stalled file held a worker forever,
  its lease renewed; ffprobe has a minute of its own.
- **ffmpeg is required like libvips**: without it render does not start. "Photos
  without videos" would render a Live Photo without its motion for good (its work
  done for that input).
- **Smoke, videos** (2026-10-08, the owner's library, ffmpeg-full, 4 workers): the
  12 plain-folder videos rendered (the 8 HEVC iPhone ones that were Waiting are
  Ready), every rendition smaller than its source (HEVC 1080×1920 → H.264 720×1280,
  rotation applied); a screen recording at CRF 23 alone came out at 14 Mb/s —
  capped at 4 Mb/s (17.5 MB → 5.4 MB).
- **A smoke run builds its plugins into its own directory**: `make build-plugins`
  rewrites `.build/plugins/*.so`, which the owner's running server has loaded — the
  `smoke` skill said so, and was wrong.

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
- **vulncheck GO-2026-5932 is not ours** (2026-10-06): `x/crypto/openpgp` is
  declared unsafe as a whole package, with no fixed version; `x/crypto` comes with
  echo (`acme/autocert`) and nothing imports `openpgp`. `govulncheck ./...` says so
  ("your code doesn't appear to call"); the gopls MCP lists it as a finding by the
  module alone. Nothing to bump.
- **One GUID type, through the perceptors** (2026-10-06): `api.GUID` in perceplib —
  an item's GUID is its main file's, an Apple asset's UUID, the key perceptors keep
  their values by: one identity, so one type, never converted inside the core. A
  string only at the edges: a URL parameter, a file path, PhotoKit (the Photos
  UUID), a log. Rejected: a type of the core only (`dto.GUID`), converted at the
  perceplib boundary — the perceptors' stores and navigation would stay untyped.
- **The nil GUID, not `"-"`** (2026-10-06): an ignored group's files linked to
  `"-"` — a magic string in an id field. Now `api.NilGUID`
  (`00000000-0000-0000-0000-000000000000`): in the GUID's format, naming no item.
  `LinkedTo` "" = not decided yet. `Open` migrates the old mark.
- **Go 1.27 has `uuid` in the standard library**: new GUIDs come from `uuid.NewV4()`
  (random, as `github.com/google/uuid`'s `New` was), one dependency fewer. Its `New`
  is left alone: it picks the algorithm, and a change of how GUIDs are made is not a
  side effect of a type. Trap: goimports resolves a bare `uuid` to it.
- **Public first, by a script** (2026-10-06): the files were reordered by moving
  whole declarations (with their comments), so the diff is moves only. Skipped:
  cgo files (the `import "C"` preamble) by hand, a spike under `_sb`.

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

- **Docker: two containers, one address** (#38, the owner: ML and Postgres will be
  services of the same compose — no point squeezing into one process): the server,
  and the web — Caddy serving the gallery as a single-page app and sending `/api`
  to the server over the compose network. The browser cannot reach a container by
  name, and the API's address was baked into the client at build time
  (`$env/static/public`): now it is `/api`, wherever the image runs, no CORS. Caddy
  rather than a proxy of our own in SvelteKit's Node server: the `/items` stream
  (`flush_interval -1`) and video ranges work as they are. The client never used
  server rendering — `adapter-static` (3.x: 4.x wants SvelteKit 3) with `ssr = false`.
- **The server image**: trixie (Debian's stable) and `jellyfin-ffmpeg8` (Jellyfin
  builds 8 for trixie, not for bookworm); the plugins built in the same stage as the
  host (a plugin loads only into a host built the same way); `libheif-plugin-libde265`
  or libvips reads no HEIC. CI builds both images and runs the whole way through
  compose (a photo made by the image's own vips, imported, rendered, served through
  Caddy) — there was no Docker on the dev Mac.
- **Client config changes in a worktree**: the owner's Vite runs from the main
  checkout; changing svebapp's config or `.env` there restarts it (a hard limit). The
  Docker work went into a `git worktree` beside it.

- **Corrections become skills and checks** (2026-10-06): over one long session the
  owner kept re-teaching the same things — the push and PR order, answering a
  review, the smoke pair, and a taste (state held twice, hand-set booleans, names,
  files by subject). Procedures went to `.agents/skills/` (read on demand, no
  context spent until then); the taste stayed rules in AGENTS.md, with
  `self-review` as the checklist that catches them; what a machine can see went to
  CI (`scripts/declorder`: the declaration order). Rejected: an MCP server for it —
  MCP gives tools access to systems, it holds no rules; it is how fleet will
  install these into other projects.

- **Libraries of their own** (2026-10-05): the chain, the logger and the write bus
  are general — [go-chain](https://github.com/eggs-gd/go-chain),
  [go-zap-decor](https://github.com/eggs-gd/go-zap-decor),
  [go-pub-sub](https://github.com/eggs-gd/go-pub-sub) — public repos with their own
  tags, as go-exiftool; perceplib keeps the perceptor contract (`api`, `exif`).
  Their own reviews found bugs the packages had carried for months: a join written
  by two sub-chains closed by the faster one (send on closed channel); a switch
  index past its outputs lost silently; `DisableService` never matched (zap's name
  carries the color codes); `With` fields bypassed the decorator (zap's core wrote
  them inline, the promoted `Clone` dropped the decorating encoder); zero values
  and durations misformatted; the stack trace between the line and its fields; a
  race in `Named`. Go plugins still need the very same versions of these as the
  host (perceplib's `api` imports go-chain and go-zap-decor).

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
