# Findings & decisions

A log of what has been learned in practice: what broke, why, and what was decided.
The goal is to not dig through the same problems twice and not re-invent the same
ideas. Add new entries at the top of a section, with a date.

Design diagrams live in [`../puml`](../puml) (rendered: [`../diagrams`](../diagrams)).

---

## Frontend: gallery layout, resize, streaming

### Resize and streaming (2026-09-29)

**Symptoms:** while streaming, clicking a photo and resizing did nothing until the
stream finished; after a resize all photos vanished (sometimes for good); tiles did
not move to their new places, they disappeared and reappeared.

**Causes (each one broke it on its own):**

- **One message per photo → O(N²) on the main thread.** Every worker message is a
  separate event-loop task, Svelte cannot batch them. For each one: full sort of
  `items` in the store, regrouping into rows, and `animate:flip` measured
  `getBoundingClientRect` of **every** tile. With 3000 photos the tab froze ~90 s;
  clicks and the `$effect` that forwards resizes to the worker never ran.
- **`reset` before a rebuild.** A full rebuild started by clearing the view → every
  tile faded out, then came back one by one.
- **Task cancellation + subscribing Dexie hooks per task.** The `finally` of an
  aborted task unsubscribed the very hook functions the new task had just
  subscribed → the UI stopped receiving updates ("photos gone for good"). Parallel
  tasks also shared mutable state.
- **One `{#each}` per row.** A tile moving to another row is destroyed and
  recreated; `flip` only works within a single `each`.
- **Debounce instead of throttle** (`clearTimeout` on every new photo): a continuous
  stream never pauses for 32 ms, so the deferred layout never ran.

**Decision (current architecture, `svebapp/src/lib/workers/tasks/wlayout.ts`):**

- The worker keeps all photos **in memory** (taken from `wsync`'s `create` messages,
  no Dexie reads) and computes the layout **synchronously**: `x, y, w, h` per photo.
  Nothing is awaited → nothing to cancel → no races.
- A resize = one full rebuild = **one `replace` message**. Nothing is cleared up
  front. While dragging the window edge: 33 ms throttle, always the latest width.
- The stream stays "one photo at a time" (`upsert` per photo) — that is the product
  concept.
- The UI (`stores.ts`) applies the message queue **once per frame**
  (`requestAnimationFrame`); several `replace` messages in one frame → only the last.
- The gallery is a flat list: `position: absolute` + `transform: translate()`, keyed
  by `guid`. On resize the same DOM node glides to its new place (CSS transition),
  including into another row. `flip` is gone.
- "Wave" on resize: tiles that stay in their row only rescale; tiles that move to
  another row start one after another (12 ms step, 150 ms cap, visible tiles only).
  The cap matters: while dragging, a new layout every 33 ms restarts pending delays.

**Verified with** a Node harness (the worker bundled with esbuild + `fake-indexeddb`,
the UI side emulates `replace/upsert`): streams of 3000 / 20 000, resize mid-stream,
a resize storm — all photos present, closed rows exactly full width, 0 overlaps.
A full relayout of 20 000 photos takes a few ms.

**Testing in a browser:** a hidden tab/pane does not paint frames, so
`requestAnimationFrame` never fires and the queue just grows. Measure animations in
a visible tab only. Streaming issues need a slow backend (a local gontroller sends
~2400 items in 50 ms — nothing is visible); a mock that streams N items with a delay
was used.

### Rejected / postponed

- **Batched messages from the worker** (one per layout pass) — works, but contradicts
  the "stream one photo at a time" concept. Optimise how the UI receives first.
- **Optimal layout (Dijkstra, `workers/layout/`)** — the original masonry algorithm:
  a graph of all possible row breaks, minimising deviation from the target row
  height. Not suitable for streaming: every new photo may rearrange rows above.
  Possible use: relayout of an already loaded gallery. Code kept; `dijkstra.js` is
  third-party MIT code (2008) under `// @ts-nocheck`.

### Visible window over layoutDb + resize anchoring (2026-09-29)

The gallery moved to the original design (`Workers.puml`): `wlayout` writes positions
to `layoutDb`, the page subscribes to the visible window with `liveQuery`; worker → UI
messages are gone. Details: [svebapp README](../../svebapp/README.md#visible-window-and-resize-anchoring).

- Only the window is in the DOM (≈40 tiles for 2400 photos, container height from
  `meta`). The window query itself takes 1–3 ms.
- **Resize keeps the view:** without an anchor the same `scrollY` shows other photos
  after a relayout (true for the previous message-based version too). The first
  visible photo is sent as an anchor, the worker reports its new position in `meta` in
  the same transaction, one `liveQuery` returns items + `scrollTo` together. Checked
  at 700 / 1000 / 450 / 600 px and a 30-step drag: the anchor photo stays in the top
  row.
- **Bug found while testing:** scrolling before the new (taller) height was in the
  DOM got clamped to the old document height (450 px case). Apply height/items,
  `tick()`, then scroll.
- **Streaming:** main-thread timer lag p50 0 ms, p95 8 ms, max 14 ms with 3000 photos
  at 5 ms each (the old per-item messaging froze the tab for ~90 s). A relayout in
  the middle of the stream (at 475 photos) → the final layout of 3000 is consistent
  (full-width rows, no overlaps, height matches).
- **Fixes after a manual test (2026-09-29):**
  - *Streaming felt worse than it should:* every streamed photo updated the total
    height in the meta record that the window query reads → the window query re-ran
    per photo (the "whole-table subscription" trap through `meta`). Now `meta` has two
    records: `layout` (rev, width, anchor — changes only on a relayout; the window
    query reads only this) and `size` (height, count — separate cheap subscription).
    Streamed photos are written in batches (one transaction per 16 ms, not per photo).
    Timer lag during a 3000-photo stream: p95 8 → 2 ms.
  - *The view drifted after several resizes back and forth:* the anchor was re-taken
    on every resize. While a relayout is on its way the screen shows the old layout at
    an already corrected scroll; and after a reflow the top-left photo is often an
    earlier one than the anchor — each burst walked the view back a little. Now the
    anchor (with a relative offset: fraction of the tile height) is taken once and kept
    across all resizes until the user scrolls. Six bursts of back-and-forth resizes:
    the anchor stays in the top row, returning to 1000 px gives the same `scrollY`
    every time.
  - Snapshots are applied on the next animation frame **or** after 100 ms — frames
    stall in windows that are "visible" but not painting.
  - *Blank tiles while scrolling* (the tile is there and clickable, the image appears
    after opening the viewer). First guess — `loading="lazy"` missing tiles moved by
    `transform` — was **wrong**: without it blanks got more frequent. Real cause: the
    gallery shows **originals** (median 0.5 MB, up to 5 MB; ~40 % HEIC) in 200 px tiles;
    decoding a window of ~80 multi-megapixel images exceeds the browser's decoded-
    image budget, it drops some and paints nothing until a repaint (the viewer
    triggers one). Lazy loading only limits concurrent decodes. Proper fix:
    thumbnails from the server (also needed for HEIC — Chrome/Firefox cannot show it).
  - *Anchor still drifted a little:* the browser's own scroll anchoring adjusted
    `scrollY` when content changed, which read as a user scroll and dropped our
    anchor. `overflow-anchor: none` on the gallery.
  - *Rows taller than the fixed 1000 px overlap* (Codex review): a portrait closed
    alone is stretched to the full width, a row can be thousands of px tall and
    vanished when scrolled past its top. Items now have an indexed `bottom`; the
    window query finds the first row reaching into the window exactly (bounded
    ranges only, so streamed photos below the window still do not re-run it).
  - Not a bug: with the mock's repeating aspect ratios a 1008 px layout is taller per
    photo than a 700 px one (rows of 3 stretched to the full width), so the page gets
    shorter when narrowed mid-stream.
- **Resize inside the gallery looked chaotic (2026-09-29):** correct end state, but tiles
  animated in page coordinates while the anchor correction scrolled at once — on
  screen everything jumped by Δ and flew back, from above and below. Fix: FLIP on the
  container (shift by Δ, animate to 0 with the tiles). Measured: the pin point stayed
  at exactly 384 px (screen centre) in all 60 samples during the animation while the
  scroll jumped 60 000 → 83 924. Product decision: the anchor is hybrid — page top
  pins the top, page bottom pins the end, otherwise the photo in the screen centre.
- **Still restless in the middle: the wave direction (2026-09-29).** The wave ran top
  to bottom by order, so mid-page and at the bottom tiles moved from above and below
  the pinned photo at once. Now the wave starts at the anchor (centre: both ways by
  order distance; bottom: upwards). This needed per-tile FLIP instead of CSS
  transitions + a container shift: with the container shift a tile waiting for its
  turn drifted with the container before its own animation. Measured: waiting tiles
  moved 0 px; with a fixed 60 ms step and a 500 ms cap only ~9 tiles formed the wave
  and the rest started together — the step is now adaptive (26 visible tiles →
  26 distinct delays, 20 ms apart).
- **Upward wave looked unnatural, widening flashed white (2026-09-29).** A wave by
  photo order goes right-to-left, bottom-to-top when it runs upwards (page bottom, the
  upper half around a centre anchor). Now the wave goes by rows (same distance above
  and below together), left to right within a row. Widening makes the layout shorter,
  so many photos that were not rendered come into view: they faded in from 0 while the
  old tiles left at once → a nearly white screen for a moment. Tiles mounted by a
  relayout now appear without the fade.
- **Considered, not done:** window/size subscriptions as `LiveQuery` in `$derived`.
  A new query per window step starts empty → tiles would disappear for a moment on
  every step unless the previous value is kept; and the per-frame application plus
  the scroll correction are side effects anyway. The subscriptions stay in `$effect`
  as the integration boundary.
- **Test environment:** the agent's browser pane often does not paint; then
  `requestAnimationFrame` runs at 0–1 fps and `ResizeObserver`/scroll events are not
  delivered. Mid-stream resize was verified by calling `updateLayout` directly
  (`await import('/src/lib/workers/proxy.ts')` on the dev server returns the live
  module).

### liveQuery across threads — spike (2026-09-29)

Spike: [`svebapp/spikes/livequery`](../../svebapp/spikes/livequery) — a worker writes into
IndexedDB with Dexie, the page subscribes with `liveQuery`.

- **Works across threads.** Dexie 4.4.6 fires `storagemutated` on every committed
  write transaction and forwards it to all same-origin contexts via
  `BroadcastChannel` (`x-storagemutated-1`); a `liveQuery` elsewhere re-runs only if
  the mutated key ranges intersect the ranges it read.

  | Scenario | Arrived | Latency worker → page | Whole-table query re-runs | Window query re-runs |
  |---|---|---|---|---|
  | 50 tx × 100 items | 5000/5000 | p50 3 ms, max 4 ms | 34 | 2 (0 after filled) |
  | 2000 tx × 1 item (stream) | 2000/2000 | p50 0 ms, max 2 ms | 1578 | 2–3 (0 after filled) |

- **A window subscription is precise:** once its range is filled, writes outside it
  do not re-run it. That is exactly what virtualisation needs.
- **Whole-table queries are the trap:** in the per-item stream they re-run on almost
  every transaction (1578 / 2000) → O(N²). The UI must subscribe to the visible
  window only; totals (gallery height, count) come from a small separate record.
- So the original failure was not Dexie. Remaining suspects: `.clear()` on import of
  `itemsDb`/`layoutDb` in every context (it can wipe rows another worker just wrote),
  the `derived` wiring. `dexie-observable` was never imported — a dead dependency,
  removed.

### liveQuery instead of messages (original design, not implemented yet)

The original design ([`Workers.puml`](../puml/Workers.puml)): workers work with
IndexedDB directly — LayoutWorker writes results to `LayoutDB`, the View subscribes
to it and redraws reactively; no messaging between worker and UI. It did not work
back then: worker writes did not trigger live updates in the UI, and it did not
compose with `derived`. The current message protocol is a deviation from this design.

- Dexie ≥ 3.2 is supposed to propagate mutations across tabs/workers via
  `BroadcastChannel` (`storagemutated`) — **verify with an experiment** on Dexie 4.
- Likely reasons it failed then: `.clear()` on import of `itemsDb`/`layoutDb` in
  every context (main thread + 2 workers) races with writes; the legacy
  `dexie-observable` add-on in dependencies; the `derived` wrapper:
  ```ts
  derived(params, ($p, set) => {
      const sub = liveQuery(() => query($p)).subscribe(set);
      return () => sub.unsubscribe();
  })
  ```
- **Caveat:** `liveQuery` over the whole table re-runs the whole query after every
  transaction → the same O(N²). It only pays off when the UI subscribes to the
  **visible window** (`where('order').between(from, to)`) — which is virtualisation
  for free.
- Implemented — see "Visible window over layoutDb" above.

---

## Navigation: perceptors and the sheet

### Every perceptor gives the sheet its order (2026-10-01)

Diagram: [`Perceptors.puml`](../puml/Perceptors.puml).

- **Navigation is a base requirement of a perceptor, not a kind of it** (decided):
  `View`/`Order` are in the base interface; if a split is ever needed it is added
  then. Every perceptor gives an honest view (size by megapixels, videos by length;
  stubs without data say so). **Sections come from the real values**, like years and
  months for the date — "12 MP" then "4032×3024", "3 min", "45 s" — not fixed bands;
  the one fixed label per perceptor is "no value" ("No date", "Unknown size", "No
  length"). On the real library: 35 megapixel sections (1 707 exact sizes below
  them, panoramas mostly), 65 lengths.
- A "videos only" filter button was considered and is not needed: the Length view
  puts every video first, longest to shortest — the slice does what the filter would.; the config decides which ones the client gets
  (`perceptors.<name>.client`, next to `enabled`).
- **A perceptor's point is a way through the library** (decided): the user always
  sees the whole endless sheet, ordered by the active perceptor around the photo
  they are at. **No filters** — one sheet in different slices. Absolute perceptors
  (date, geo) ignore the anchor; relative ones (faces, objects, similar) build a
  **two-sided trail**: from the anchor to the nearest unseen photo, then the nearest
  to that, both ways — a distance is a ring around the photo, not a line; the trail
  makes a line where neighbours are really alike (the name: Perceptrail).
- **The server orders, the client lays out** (decided). Perceptors are Go plugins on
  the server — ordering on the client would mean writing each one twice; embeddings
  do not fit a browser (2 KB per photo × 100k) and need a nearest-neighbour index.
  The layout stays in the client's worker: it depends on the window width (a
  server-side layout was considered earlier and moved to the client for that).
- **A photo may start several sections — a path** (`sections: [{level, label}]`,
  coarsest first): a year starts its first month too, a region its first city. With
  one section per photo the first month of a year and the first city of a region had
  no mark of their own (the tip said "Europe", not "Athens Europe"). A perceptor may
  give a whole path of tags or a single one.
- **Sections ride in the order stream** (`{guid, section?: {level, label}}` on the
  first item of a section), not a separate list: the panel's positions come from the
  layout (the client's), and a relative perceptor's sections depend on the anchor.
- **Geo is not one axis**: west → east puts Krakow next to Cape Town. A Hilbert curve
  maps (lat, lon) to one number that keeps near places mostly near (planned).
- A switch reuses the resize anchor: the worker relays out with `anchor`, the window
  query scrolls so the anchor keeps its screen position; from the viewer the anchor
  is the viewed photo with ratio 0.5 (centred). Closing the viewer then must not
  "reveal" the photo by its old position — the gallery skips it when that photo is
  the pending anchor.
- **The side panel's scale is by sections, √ of their photos** (decided). Linear by
  the sheet's height, a big "no value" section (two thirds of a library without a
  place) took two thirds of the panel and the real places' labels did not fit. A
  fixed small share for it (10%) was rejected — it flips the problem: two thirds of
  the library in a tenth of the scale. Now every coarsest section gets a share by √
  of its photo count (and room for its label when that fits), **at every level**:
  the sections share their parent's part of the track the same way (years, then
  months; regions, then cities). "no value" is not special, just big. 30 places × 30
  photos + 6 000 without: "no value" 87% → 32% of the panel. Only the top level was
  not enough: on the dev library geo has two regions ("Europe", "No place"), and Kyiv
  (3 047 photos) took the whole of Europe — the other cities sat in its first 12 px;
  split by √ inside the region too they spread over 0–111 px.
  Sections that start in one row (a few photos each — Sofia, Athens and the start
  of Tirane share y = 0) have shares of their own but no height on the sheet: a
  label is placed at its share's start (not at `toTrack(y)`, which put them all at
  one point and the region's label at the start of the last one), marks are ordered
  and assigned to shares by photo, not by y. The tip under the pointer the same: by
  each section's share on the track — by y every earlier city of such a row showed
  the last one's name (Codex review). Linear within a section, so scrubbing stays smooth; the view marker gets
  taller where photos are sparse. Considered next: a magnifier around the pointer
  (like the macOS Dock) for very large libraries.
- **Every rearrangement uses the wave** (decided: the wave is the product's style). A
  switch first replaced the screen at once — it reused the resize relayout, where new
  tiles appear without a fade (a fade from 0 left the screen empty when widening),
  and on a switch almost every tile is new. Now the switch's relayout fades the new
  tiles in by the same wave as the moves: from the anchor outwards, row by row,
  within 500 ms; the anchor and shared tiles move as on a resize.
- **Views live in the URL** (`/v/<view>?at=<guid>`, `/v/<view>/<guid>`). Two traps on
  the way:
  - `?at` is kept up to date with a shallow `replaceState`; Back to such an entry gives
    `page.url` **without** it (the URL of the original navigation) while the address
    bar has it — `?at` is read from `location`.
  - Back restores the entry's scroll (the router), which looked like the user's
    scroll and dropped the switch's anchor: while a switch is on its way, scroll
    events do not drop it. Buttons navigate with `noScroll`.
  - In a hidden browser pane no scroll events fire: tiles measured in the DOM are
    stale — check positions against `layoutDb` instead.
- **Why every load looked like a fresh database** (Dexie was there all along): the
  sync cleared itemsDb at its start, and the layout worker dropped all items on
  `sync-start` — a direct link waited for the stream to reach its photo. Now the
  items are kept and synced by a delta (`/items?since=` + tombstones, an epoch for
  the database). Traps on the way:
  - SQLite keeps times as text with the writer's offset (`…+03:00`, fractional parts
    of any length; some `deleted_at` with no zone at all): compared as text they lie.
    `julianday()` compares them; deletions are asked a day earlier (a tombstone too
    many is harmless).
  - A test that opened the `items` IndexedDB and never closed it blocked Dexie's
    upgrade to the new version — every later open waited. Close connections opened
    by hand.
  - A refresh of the order after a sync must not run during a switch: it aborts the
    switch's request, which then counts as failed.
  - The cursor is the stream's **last line** (`{cursor}`), not a header (Codex, PR
    #18): once the 200 and its headers are out, a DB error mid-stream cannot turn into
    an error status — the client would keep a header cursor from a cut-short stream
    and never ask for what it missed. No last line = cut short = the old cursor stays.
    Likewise a line that cannot be parsed or stored fails the sync (it was only
    logged — the cursor moved past the item).
  - **Two tabs broke the copy** (2026-10-02): every tab has its own sync worker over
    the one IndexedDB. A full sync in one tab cleared the table while another was
    filling it, and that one kept its cursor over what was left — Chrome showed 79
    photos of 6 993 and the deltas never brought the rest (Safari, Cursor: one tab,
    fine). Now syncs take a lock shared by the tabs (Web Locks,
    `navigator.locks.request('items-sync')`, works in workers), and the stream's last
    line carries `total` (shown items, counted with the cursor): after a delta a copy
    holding another count drops its state and syncs from nothing. A full sync that
    differs only warns (what changed while it ran comes with the next delta) — no
    loop. Checked: 79 kept of 6 990 → healed on reload; two tabs from an empty copy →
    6 990.
  - **The layout was shared too**: `layoutDb` (positions, the side panel's sections,
    the views' kept orders) was one database for all tabs, each tab's layout worker
    rewriting it for its own width and view — a date tab next to a place tab showed
    the place view's sections ("Europe"). Now one layout database per tab
    (`layout-<random>`, named by the page at load, passed to its worker with `init`);
    the page holds a Web Lock of that name while it lives, and a new page deletes
    the `layout-*` databases nobody holds (closed tabs, earlier loads) and the old
    shared `layout`. The views' kept orders are the server's data, the same for every
    tab: they moved to `itemsDb.orders` (v5). Not sessionStorage for the name: a
    duplicated tab copies it, and two tabs would share again. Checked: a 1024 px
    date tab and a 500 px place tab — each its own width and first mark ("2026" /
    "Europe"); a closed tab's database gone on the next load; the viewer's direct
    link and arrows.
  - **Changes reach every tab** (Codex, PR #20): the sync told only its own tab's
    layout worker (a private MessageChannel). With the lock, a second tab's sync
    finds the cursor already moved, gets an empty delta and keeps the items it
    loaded. Now wsync broadcasts every change on a BroadcastChannel
    (`items-changes`) and every tab's layout worker listens. Checked: an item hidden
    on the server, one tab synced — both tabs' layouts lost it.
  - **Plain HTTP on a LAN address** (Codex, PR #20) — the likely self-hosted setup —
    is not a secure context: no `navigator.locks`, no `crypto.randomUUID` (the client
    would have died on load). Without Web Locks syncs run unserialized (the count
    check heals what two tabs break), live pages answer a roll call on a
    BroadcastChannel instead of holding a lock (a page whose database is dropped all
    the same reloads itself), the database id is not a UUID. Checked on
    `http://192.168.x.x` (`isSecureContext` false): full sync, layout, earlier
    databases dropped, two tabs kept. The browser pane blocks a LAN page's requests
    to another port (`ERR_BLOCKED_BY_CLIENT`): tested with Vite proxying the API
    on the same origin.
  - Switching to a view with a kept order shows that order at once; if the refresh
    from the server then fails, the switch stands on the kept order (the URL was
    being rolled back while the sheet already showed the new view).
- A perceptor's icon is SVG from a plugin: shown as a CSS `mask-image` — no script in
  it runs, and the button's colour paints it (`currentColor` does not reach an
  `<img>`).
- `routes` cannot import the plugin manager (`app` imports `client` for its config):
  `main` passes the loaded perceptors to the web service.
- `LiveQuery.current` read outside an effect or a template (a key handler) is
  `undefined` — the subscription is only active while something reactive reads it.
  The viewer's bound check queries Dexie directly.
- The layout turns pointer events off for the whole `<main>` under the viewer (only
  `.viewer` turns them on): anything fixed outside `.viewer` (the toolbar) must turn
  them on itself, or clicks fall through and close the viewer. `element.click()` in a
  test does not hit-test — check with `elementFromPoint` or a real click.
- Testing on a second server started from an agent's shell: macOS does not let that
  process read `Photos Library.photoslibrary` (privacy), so Apple files are 404
  there while the user's server serves them.

### Perceptor data: the core keeps it (2026-10-01, design)

Design: roadmap "Perceptor data"; diagram [`Perceptor data.puml`](../puml/Perceptor%20data.puml).

- **Rejected: every perceptor opens a database of its own.** A perceptor is a `.so`
  in our process: its own database means a driver inside the plugin — `go-sqlite3`
  is cgo, and Go plugins already need identical versions of everything; two copies of
  a cgo driver in one process is asking for trouble. Every plugin would also need the
  data dir and, for Postgres, the credentials. And the core would not know about the
  data: a deleted photo would leave rows behind, a new plugin version would not know
  what to recompute.
- **Rejected: an API for plugins to create tables** — the same, one step removed:
  SQL in plugins, the core blind to the data.
- **Decided: the plugin declares, the core keeps.** The core creates, migrates and
  maintains the storage (deletions, schema versions), and picks the place by driver.
- **SQLite: a file per perceptor**, not everything in the main database: ML
  perceptors (embeddings) would bloat it; SQLite has one writer per file, so a
  perceptor writing does not block the import; a perceptor's data is reset by
  deleting its file. No `ATTACH` (default limit 10, per connection — awkward with
  GORM's pool): the core reads a perceptor's values by a list of guids. Postgres:
  one database, a schema per perceptor.
- **Typed API, not `SetValue("lat", …)`**: string keys and `any` stuck out of the
  plugin API. A plugin declares a struct (`api.NewStore[Location]`) and gets `Put` /
  `Get` of that type; the untyped exchange stays between `perceplib` and the core.
- **Groups** (`ProcessingMode` Group — clusters, albums, journeys) are data of
  another shape: groups of photos with data of their own, decided over the library,
  not per photo in the import chain. Designed (roadmap), not in work.
- Implemented (PR #17) for Single, with geo. Non-obvious on the way:
  - **"Nothing found" is a row** (`has = 0`): an item without GPS must not count as
    "not processed by geo", or every walk would send it through exiftool again.
  - **Processed again by asking the store**: the files gate checks `Has(guid)` in
    every import perceptor's store — a new perceptor or a changed schema takes one
    pass over the library, while the items stay shown (marking them Dirty would hide
    them until processed).
  - **`rows.Err()` after reading a store**: an SQLite error mid-read only ends
    `Next()` — without the check `Load` returned a partial map as if whole, and the
    sheet came out quietly wrong (Codex review).
  - **Deletions are pruned after a complete walk** (the store's guids against the
    items), not hooked into every delete path.
  - **Geo sections from the time zone** of the place (tzf, already in the core for
    dates): real places without a geocoder dataset — "Europe" → "Kyiv". Country
    names need an offline geocoder (roadmap).
  - **Every city in one piece**: by the curve alone a city came back several times
    (the curve zigzags through a region — Tirane ×3, Kyiv ×2 on the dev library).
    Now regions and cities are ordered by their first point on the curve (near ones
    stay near), photos within a city by the curve: each city and region is one span
    of the sheet (Athens → Tirane → Skopje → Podgorica → Zagreb → Budapest →
    Bucharest → Kyiv → Minsk).
  - **A plugin's dependencies must match the host's exactly** — tzf brought older
    `golang.org/x/sync` / testify into `exif_geo`; aligned by hand. GPS parsing moved
    into `perceplib` (`api.Coordinates`), shared by the core's date and geo.
  - On the dev library from an agent's shell only the 575 generic items went through
    (the Photos library is unreadable there): 12 with a place, 4 of them shown (8
    are Waiting HEIC/HEVC).

### Transcode: decisions before the code (2026-10-01, design)

Design: roadmap "Expensive stage".

- **No release without the transcode** (decided): a release is a Docker image someone
  installs; without the transcode HEIC does not show in Chrome, iPhone HEVC video does
  not play, big originals slow the grid. 0.2.0 = transcode + Docker (the earlier bar,
  "the first perceptor end to end", was met by geo and is not enough).
- **One codec, one image format, chosen in the config** — not arrays. Arrays let the
  browser pick (the asset contract is ready for it) but cost N× disk and transcode
  time; a later feature of its own.
- **Sizes: any array in the config, `[400, 1600]` to start with.** 1600 px is less
  than a 2K 32" (2560) or a MacBook 16" (3456), and tiles would want 800 on Retina —
  but ~3840 is close to the original itself: we make previews, not copies; the
  original stays behind the viewer's Original switch. Larger sizes are a config
  experiment, the system must take any array.
- **How others split it**: Immich — one server image does API, previews and ffmpeg,
  the same image can run as workers (env), ML in its own container; PhotoPrism,
  Jellyfin — monoliths, ffmpeg as a subprocess; LibrePhotos — backend + queue
  workers. Common ground: previews and transcode in the core, ML apart. Ours: in
  gontroller by default, roles later from the same binary — the DB-state queue makes
  that free.
- **Hardware on the dev box**: i5-13500T (Raptor Lake, UHD 770) — QSV encodes and
  decodes H.264 / HEVC 8/10 bit, decodes AV1, does not encode it. Docker on a Mac has
  no GPU: VideoToolbox only in a native binary.
- **jellyfin-ffmpeg** in the image rather than our own build: every hardware backend
  and HDR tone mapping (iPhone HLG / Dolby Vision would come out washed out in H.264
  without it).
- **Software first, hardware as a step of its own** (decided): the queue, the sizes,
  the outputs and the HDR tone mapping are the same for any encoder and are got right
  once on `libx264` — everywhere, CI included (no GPU); the software path stays the
  reference and the fallback. Hardware is tied to Docker (QSV: `/dev/dri`) or to a
  native binary (VideoToolbox on a Mac), so it comes with or after Docker. What is
  needed from day one: the codec → encoder table and the probe with a fallback.
- **Photos: CPU first, a hardware decoder only if measured** (discussed with ChatGPT
  too). Hardware wins for video (fixed-function decoders/encoders); for photos it
  helps parts of the pipeline at best — JPEG through libvips shrinks while it loads,
  and copying a 24 MP bitmap to the GPU and back can eat the gain. Immich renders its
  photo previews on the CPU too (hardware transcoding there is video only). For this
  library the bigger win is the source: Photos' JPEG, the HEIC's embedded thumbnail,
  the full original last. Benchmark (images/s, CPU, peak RSS) before adding a macOS
  ImageIO HEIC decoder; its hardware path is an assumption to check.
- **Apple Photos: one copy** (2026-10-02, decided; roadmap step 0). A cache of our
  own beside the library does not save disk, it adds a second copy — whatever Photos
  keeps is served as it is, what it lacks is asked of Photos (PhotoKit) and lands in
  its library, under its storage policy. At most a bounded working set (LRU, size
  limit). By level, on demand: small renditions ahead for the tiles, the medium one
  (~2048, ~1 MB — ~40 % of the originals for a whole library) when a photo opens,
  the original on its button. Our webp at 1600 would be ~4× smaller (fewer pixels,
  q80 against Apple's ~q90+, a better codec) — a saving on a duplicate, so no reason.
- **"Download Originals" — one option, not the only strategy** (the user): for most
  libraries one checkbox makes everything local and is the simplest; for decades of
  photos it forces the whole library onto the disk, and Photos has no "renditions
  local, originals on request" mode — all or its own choice. Asking Photos per level
  is that missing mode, for the libraries that stay on Optimize Mac Storage.
- **PhotoKit from Go, not a Swift helper**: it is Objective-C, cgo calls it in the
  gontroller process (cgo is there for the plugins anyway), behind
  `//go:build darwin`. Open for the spike: the Photos permission for a bare binary
  (Info.plist through `-sectcreate`), whom it is granted to when started from a
  terminal, and whether a rebuild (new ad-hoc signature) drops it.
- **Reading the library stays in every build** (`groups/apple` is pure Go): an
  archived library after leaving Apple, an external disk, a NAS copy. Only asking
  Photos is native macOS. Docker on a Mac is a Linux VM — no PhotoKit, and no access
  to the `.photoslibrary` without Full Disk Access: a Mac with Photos runs the native
  binary.
- The agent's shell cannot read `Photos Library.photoslibrary` (macOS privacy): the
  JPEG quality of Apple's renditions was estimated from the sizes in the DB
  (~2.6 bits/pixel), not measured — the spike has the same permission question.

### Plugins and RAW previews: two holes (2026-10-02)

- `TestLoadExternalPlugins` loaded a `test_plugin.so` that did not exist and only
  logged — a plugin built against other versions than the host was caught only by
  running the server. Now it builds each perceptor and loads it. Checked: a plugin
  pointed at a perceplib copy with one constant added fails with "plugin was built
  with a different version of package". Traps:
  - `go test` caches the result and ignores files outside the test's module, even
    the ones the test stats itself — `perceptors/` and `perceplib/` are other
    modules. After changing a plugin: `-count=1`; CI does that after
    `make build-plugins` (setup-go keeps the Go cache between runs).
  - The plugin must come from the same Go as the test binary: the test runs
    `$(GOROOT)/bin/go`, not whatever `go` is on the PATH.
- An embedded RAW preview (`JpgFromRaw` / `PreviewImage`) is stored as the sensor
  saw it, with no EXIF of its own: extracted as is, a portrait shot lies on its side.
  The RAW's Orientation is copied onto it (`-TagsFromFile`); the browser turns an
  `<img>` by its EXIF, and the item's size is already swapped by the size perceptor.
  Tested with a real exiftool (a JPEG with an embedded thumbnail stands in for the
  RAW — there are no RAWs in the dev library); skipped where exiftool is missing (CI).

### A viewer over a library, not a library (2026-10-02)

- The product's place (the owner): Immich, PhotoPrism, Lightroom, Apple Photos keep
  the library — sync, albums, tags, the heavy UX. Perceptrail is another way through
  the same photos, a rediscovery of them, not a replacement. So we read what those
  libraries leave (Photos' DB, the sidecars Immich / PhotoPrism / Lightroom write) and
  never become a second copy of it (see "Apple Photos: one copy").
- Hence the grouping collects the whole asset package — originals, edits, RAW +
  JPEG, sidecars — and an item is meant to be merged from all of it (roadmap "The
  asset from all its files"). Today the metadata comes from the main file only.
- The optimal (Dijkstra) layout does not reorder photos, it only picks row breaks;
  what ruled it out is that it needs the whole set (one photo may move the rows
  above), not the views' fixed orders.

## Backend: gontroller, plugins, exiftool

### Broken files (2026-10-01)

- exiftool reads a broken file and says so: garbage and empty files give only the
  File tags plus `Error` ("File format error", "File is empty"); a JPEG cut after its
  header gives FileType / MIMEType but no size at all, with a `Warning`. Those are the
  signals: such a main file is not an item, and its files are ignored (`LinkedTo =
  "-"`) — the gate skips them until a file's size or mtime changes, then they are
  processed again. Before, they became 0×0 items or failed every walk.
- A group exiftool returns nothing for (a timeout, a crash) is still an error and is
  retried — it may pass. Apple assets are not judged by their file (the DB is the
  truth and the derivatives may be fine).
- The test's fake exiftool now returns a size, like a real image (with the new rule a
  sizeless image is broken).

### The asset contract (2026-09-30)

- **The client gets the whole asset once and decides** (decision): files by role
  (original, edit, stills, motion, frames) with size, mime and a video's codec. The
  browser does the choosing itself: `<picture>` with a `<source type>` per format
  and `srcset` widths + `sizes` = the tile's pixel width (it picks the format and
  the size, retina included); `<video>` gets `<source type='video/mp4;
  codecs="hvc1"'>` — MOV is offered as MP4 (same container family; Chrome plays it
  but does not claim `video/quicktime`), unplayable sources are filtered with
  `canPlayType` up front.
- Hover: a playable video, else Apple's video frames as a flip-book — turned by an
  attachment on the `<img>` (no component state per frame). Viewer: `sizes=100vw`,
  "Original" opens what the browser shows and downloads the rest (HEIC in Chrome).
- **The asset's kind (photo / live / video) marks moving tiles** (2026-10-01). Roles
  cannot tell an Apple Live Photo from a video: in both the original is the `.mov`
  (the source) and a photo is a still. The Photos DB says it: `ZKIND` 1 = video,
  `ZPLAYBACKSTYLE` 3 = a Live Photo with live on (`ZKINDSUBTYPE` 2 with style 1 =
  live switched off: a still in Photos). The real library: 351 live, 766 videos.
  `ZKINDSUBTYPE` 101 is not slo-mo here (Android screen recordings carry it), 103
  = screen recording — not used. Stored on the item (`Kind`) from a keyed source
  and in the meta hash (the gate reprocesses Apple assets once); generic items get
  it from the roles in the API (a video original = video, motion = live).
- **Most videos of an iCloud library are not local** (2026-10-01): 766 of 770 —
  the `.mov` is only in iCloud; Photos keeps stills and 0–10 scrubbing frames
  (`cvt/`), so hover shows a flip-book or nothing and the viewer plays nothing; a
  transcode has no source. The tile marks it (a cloud next to the kind) and shows
  the length (`ZDURATION`, written to the meta record as exiftool's `Duration`; the
  core `exif_duration` plugin reads it for every source). A real video needs
  "Download Originals" in Photos, or a macOS helper (backlog).
- **Photos' "Optimize Mac Storage" previews look good but are not originals**
  (2026-10-01): 3 528 of 5 866 photos have no local original. Their biggest local
  file is 480 px (1 627 of them), 1536×2048 (`_1_102_o`, 985) or full size for
  screenshots (2622/1206 px, ~360 — still a JPEG, the PNG original is in iCloud);
  the originals are mostly 4032 (12 MP) or 5712 (24 MP) on the long side. E.g.
  IMG_4654.HEIC: 3024×4032, 2.4 MB in iCloud; local 1536×2048 JPEG.
- **Original in the viewer is a switch** (2026-10-01): the original image replaces
  the preview in place; the browser decides by loading it (HEIC shows in Safari,
  fails in Chrome) — on an error the button becomes "Download original". A video or
  RAW original is downloaded.
- **The viewer never stretches an image** (2026-10-01): with `srcset` + `sizes=100vw`
  the browser sizes an `<img>` as 100vw whatever the file, so `width: auto` +
  `max-width/height` stopped capping it and every preview filled the screen. The
  box is now sized from the biggest image the asset has: `min(w px, 100vw, 100vh ×
  w/h)` with its aspect ratio — a 360 px preview stays 360 px, a big one fits.
- **Blank tiles were `loading="lazy"`**, measured on the real library: the gallery
  renders a window with a margin (1 viewport above, 2 below) exactly so images load
  before they scroll in — lazy loading held that margin back. A scroll pass: 37
  tiles blank 400–800 ms → 0 over 300 ms with `eager` (bounded: only the window
  is rendered); resizes: ≤ 50 ms. What is left: a far jump (one frame), generic
  assets whose only image is a big original (decode; the photo transcode fixes it),
  a video with no stills (its metadata loads first).
- Live check on the real library (a build before the contract): 6 420 Apple assets as
  items keyed by UUID, all dated from the DB, 2 987 old generic items of the
  library removed; 6 985 Visible, 9 Waiting (HEVC/HEIC outside the library).

### Apple Photos grouper (2026-09-30)

- **No 1 -> N step needed** (decision, after proposing one): the files of an asset are
  scattered over the bundle, but the grouper forms the groups up front from the DB
  and a `stat` of every candidate path — the expected files are exactly those on
  disk, so each walked file closes at most one group. A file vanishing mid-walk
  (Photos purging a derivative) leaves a group incomplete: it is held back with the
  marker (`FileGroup.Held`), the gate does not count its files as gone, the next
  walk reloads the DB.
- **The asset UUID is the item's GUID** (`FileGroup.Key`): the main file of an asset
  changes (a derivative while cloud-only, then the downloaded original) — the item
  stays. Every file links to the key; an item with no files left is deleted (the
  main/sidecar rule does not apply to keyed files).
- Synthetic run (a copy of the dev library's real `Photos.sqlite` + the real file
  list filled with sample media, since this process cannot read the library): 6 420
  assets with local files loaded, 6 417 items Visible (3 were fixture PNGs left
  empty), first import 27 s, 0 errors. Previews: small thumbnail 2 634 (videos pick
  it over `.THM`, rightly: `.THM` is 32×32, the thumbnail ~360×640), ~2000 px 1 658, original 1 157 (JPEG originals; HEIC ones show
  Apple's JPEG), ~1000 px 640, the edit 328.
- A keyed asset is media if any of its files is (a broken original still has
  Apple's derivatives).
- **The Photos DB wins over the files' EXIF** (decision): it is what the user sees
  and may have corrected in Photos; a cloud-only asset has nothing else. Date
  (`ZDATECREATED`, Core Data seconds since 2001 UTC) + `ZTIMEZONEOFFSET` (known for
  6 456 of 6 457 assets), `ZWIDTH`/`ZHEIGHT` (already oriented: they are swapped
  against the original for orientation 6 — so the record says Orientation 1), GPS
  (`-180` = none; 3 242 assets have it). Synthetic run: all 6 420 items dated from
  the DB, real sizes.
- **`ZDATECREATED` is declared `TIMESTAMP`**: SQLite keeps whole-second values as
  integers (1 361 of 6 457 here) and the Go driver turns such values into
  `time.Time` — scanning into a float failed and would have dropped the whole
  library. Read it as `CAST(… AS REAL)`.

### PhotoKit spike (2026-10-02)

Roadmap step 0. `_sb/spikes/photokit`: Go + cgo (Objective-C), the Info.plist put into
the binary by the linker; run from the user's terminal on the dev library.

- **Photos normalises the library.** Three unedited HEICs, cloud-only (original and
  medium both `-1/1`): `requestImageForAsset` ≤ 2048 px with network allowed returned
  1536×2048 in **0.92 / 0.59 / 0.78 s**; after it recipe 65741 is `1/1` in the DB and
  `resources/derivatives/<X>/<UUID>_1_102_o.jpeg` exists with exactly its
  `ZDATALENGTH` bytes (956 659 / 848 806 / 803 251). The original (recipe 0) stays in
  iCloud. A request without network right after: the image at once.
- Without network a cloud-only asset fails at once: `PHPhotosErrorDomain` 3164
  (network access required), `PHImageResultIsInCloudKey` — a free "is it local".
- Photos serves the best local thing first: an edited HEIC whose edit
  (`FullSizeRender.heic`, recipe 65938, 1991×2557) is local got the edit scaled to
  2048 in 0.16 s, nothing downloaded.
- Recipe 65741's file is `_1_102_o.jpeg` (the grouper's `roleLarge2`), not
  `_1_105_c.jpeg` (that one is recipe 65747, ~768×1024). The tiles need no request:
  `masters/<X>/<UUID>_4_5005_c.jpeg` (~100 KB, 360×480) is local even for cloud-only
  assets.
- `PHAssetResource` lists only the original, the edit and its adjustments — not
  Photos' derivatives: whether a rendition is local is read from the DB / disk (or
  answered by a request without network).
- **The permission goes to the terminal** that starts the binary (its "responsible"
  app): a terminal with Photos access ran it without a prompt, a new one asked for
  the terminal itself. So a rebuild does not drop it; the embedded Info.plist does not
  matter there. Not started from a terminal (launchd) — still to check.
- **Videos and Live Photos — the same: Photos normalises the library**, the file
  comes as `file://…/resources/derivatives/…` (not streamed); the original stays in
  iCloud unless the mode asks for it. By `PHVideoRequestOptions.deliveryMode`
  (short iPhone videos, cloud-only):

  | mode | recipe → file | codec | size | time |
  |---|---|---|---|---|
  | fast | 131081 → `_2_4_o.mp4` | H.264 640×360 | ~0.7 MB | 0.9 s |
  | medium | 131475 → `_2_201_o.mov` (iPhone), 131079 → `_2_3_o.mp4` (others) | **HEVC** / H.264 720p | 2–5 MB | 1.2–1.6 s |
  | automatic, high | the original → `originals/<X>/<UUID>.mov` | HEVC 1080p | 8–12 MB | 2.3–2.8 s |

  For an iPhone video no mode hands over the H.264 720p (131079), though iCloud has
  it. Automatic / high download the original (it becomes local): avoid them except
  for the Original button. A Live Photo (`requestLivePhotoForAsset`, 0.9–1 s) makes
  its motion local: 131275 → `_2_101_o.mov`, H.264 ~650×870, ~1.8 MB; the original
  `.MOV` stays in iCloud.
- **The `cvt` frames** (`derivatives/cvt/<X>/<UUID>/…_cvt_tNNNN.jpeg`, 400×600,
  ~48 KB) are not a recipe in the DB and no request makes them: they come from
  Photos' own background analysis. Their number follows the length (up to 10;
  0–2 for a few seconds), and a third of the videos have none, whatever the length;
  Live Photos never.
- The grouper does not know the video renditions' names yet (`_2_3_o.mp4`,
  `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`): the walk would not find what Photos
  downloaded.
- **HEVC in browsers, 2026** (checked on the user's word: "as basic as H.264 by
  now"): Safari always; Chrome / Edge since 107 on Windows and macOS (hardware
  decode); Firefox 134 Windows, 136 macOS, 137 Linux (VA-API, MP4 only). Gaps: Chrome
  on Linux (VA-API only, extra packages and flags), no software decoder in Chrome or
  Firefox (old hardware), some Windows installs need the HEVC extension. So HEVC is
  the viewer's default; the 360p H.264 we fetch for the sheet covers the gaps; no
  H.264 720p transcode of our own.
- **Videos, decided** (the user): the sheet's hover needs only `fast` (360p),
  asked for on hover; when both the video and `cvt` frames are there, hover plays
  the video. `medium` on open, as a photo's medium rendition. A Live Photo's motion
  likewise: asked for on hover, played on hover.
- **On demand, never in bulk** (the user): the sheet has its tiles already, the
  medium rendition is needed only when a photo opens — that request triggers the
  download, the viewer swaps the image in when it comes; 1–3 rows around it are asked
  for ahead. So the limits of asking for thousands were not measured: nothing will.
- **Assets with nothing local** (Codex, PR #21): on-open-only would never reach
  them — without a viewable file an item is Waiting, not on the sheet, so nobody
  opens it (7 of 6 427 in the dev library). `HydrateWaiting` asks Photos for their
  image in the background (every minute, each asset once per run); the next walk
  shows them. Not bulk: only what cannot be shown at all.
- **First run on the owner's server — nothing changed in the UI** (PR #21): the
  server sent `asset.onDemand`, but a delta brings only changed items — the client's
  kept copy never got the new field, so nothing asked. The sync epoch now carries a
  contract version (`items.go` `contractVersion`): a change to what an item carries
  bumps it, and every client takes everything once — **in place** (the owner: "this is
  critical"): the epoch is `<database>.<contract>`; another contract over the same
  database puts every item over the kept copy and deletes, after the stream, the ones
  it did not bring — the sheet never empties, a direct link keeps working. Only
  another database starts from an empty table. The count check heals the same way
  now. Checked: a new contract — the layout stayed at 6 991 rows through the sync, an
  item the server did not have was deleted; another database — emptied and refilled.
- **An edited Live Photo's motion is `_2_101_a.mov`** (`_a`: of the edit), not
  `_2_101_o.mov`: the owner's first hover on one gave a 404 — the file was there under
  the other name (with `_1_102_a.jpeg`, the edit's still). The `_a` renditions win
  (Photos shows the edit); for videos `_2_3_a.mp4`, `_2_4_a.mp4`, `_2_201_a.mov` are
  assumed by analogy, not seen yet.
- **Chrome says no to a bare `hvc1`**: `canPlayType('video/mp4; codecs="hvc1"')` is
  `""`, with `hvc1.1.6.L93.B0` it is `probably` (and it plays). The client asked for
  `?hevc=0` and got the 360p — "videos are always small" — and counted local HEVC
  files as unplayable. The server knows only the FourCC (exiftool's
  `CompressorID`): the client asks for HEVC as Main profile, level 3.1.
- **The viewer's video sized as a photo** (the owner): it had no box — its natural
  size, the stretch switch did nothing. Now a box like a photo's (its pixels once
  loaded, as big as the screen until then; stretched if switched on). A Live Photo's
  motion was 300×150 in a corner: the stage's `.stage video { width: auto }` beat
  `.live { width: 100% }` on specificity — `.asset.view video` fills the box now.
- **The viewer picks from what is here; the cloud means "full resolution not here"**
  (the owner, decided). The asset carries its full size (`full`: the current
  version's, oriented — contract 6). Opening: the smallest local image covering our
  comfortable size (the preview's ~2048 px long side, or the full size if smaller)
  is shown as it is — no 360 px first, no request; only when nothing here is that
  big is the medium asked for. As big as the full size: it is the Original, the
  switch is lit (nothing to switch to); else the switch asks for it. A video whose
  original is here and plays: played at once, lit. The tile's cloud: no file of the
  asset is as big as the full size (an original, an edit's render, a full-size
  derivative all count). Info → Files lists every file of the group (sidecars too,
  `/items/:guid/files`), each downloadable (`?download=1`: `Content-Disposition`,
  the download attribute does not work across origins). Checked on the test pair: a
  local 1600 px original — shown at once, lit, "full resolution here"; a cloud-only
  photo with 480 px local — shown, the medium asked over it, the switch off.
- **A full-size derivative is not the original**: a cloud-only PNG screenshot
  (1 206 × 2 622) has a local JPEG derivative of the same size (recipe 65739,
  `_1_101_o.jpeg`): the Original (Photos' current version at full size) is drawn
  from it, the PNG is not downloaded — the cloud stays, as it says where the
  original file is. And the request took 10 s: the asset was processed again, the
  gate dropped it (nothing changed) and the request waited for an item that never
  came. The gate tells now which keyed group it dropped (`dropped`), `Refresh`
  answers at once.
- **The cloud went only on F5** (the owner): the server had the original, a reload
  showed it, the live update did not. Dexie's `updating` hook gives the changes by
  key path (`{"asset.original": …}`); `{...item, ...mods}` put them beside the item as
  keys with dots and left `asset` as it was — fine while only top-level fields
  changed (date, size), lost for anything inside the asset. The changes are applied
  by path now (`Dexie.setByKeyPath`). Checked: an original given to an item on the
  server — the tile's cloud went with the delta, no reload.
- **A cached answer looked like a bug**: after the Original became the current
  version, the owner still saw edited screenshots unedited — the browser served the
  old answer of the same URL (cached for a day); the server's own answer matched the
  edit's render (checked pixel by pixel). The contract version is part of the
  on-demand URLs now (`?v=5`; the change itself is contract 5, so the kept items get the new URLs): what they answer changes only with the contract, and
  a new one is a new URL.
- **The Original is the biggest of what the user sees** (the owner, decided): edits
  and their history are the library's feature, not ours — we do not follow each
  provider's specifics. From Photos: its current version (the edit, cropped) at full
  resolution, asked for every Photos item (a local unedited original is not what is
  seen); elsewhere the biggest edit, else the original. The unedited original and
  its file download (`?file=1`) are gone (contract version 4).
- **The cloud and the Original** (the owner asked what each means): the tile's cloud
  meant "no original" for a photo but "no video at all" for a video or a Live
  Photo — it vanished after the first hover. Now one meaning for every kind: the
  original is only in iCloud. The Original switch: a local original the browser
  shows; otherwise Photos' (`rendition/original`: the unedited original drawn at
  full resolution as JPEG — so a HEIC shows in Chrome too — and the file itself on
  `?file=1` for the download); a Live Photo's photo always comes from Photos (its
  own original in the DB is its video); a video switches to its original file. Each
  makes Photos download the original into its library — the cloud goes after the
  next walk. Contract version 3 (`onDemand.original`).
- **"I showed the original and the cloud is still there"** (the owner): the walk did
  pick the originals up (items updated a few minutes later), but it walks a minute
  after the last one was processed, and the client asked for a delta only on a
  navigation or a return to the tab — back in the list before the walk, nothing
  told it later. First fix (a successful request cut the walk's pause short, the
  page polled every 20 s) — rejected by the owner: a whole walk for one known
  photo. Now **one asset is processed again**: the Apple grouper keeps the DB rows
  of its last load and forms that asset's group from them and the disk now
  (`Regroup` — no walk, no DB read; the DB's metadata still wins over the files'
  EXIF); the importer hands it to the files gate like any group (`Refresh`: only
  what changed passes; deletions are untouched — they come with the walk's marker;
  a walk sending the same asset at the same time processes it twice into the same
  item) and waits until the item leaves the closer. The Original's answer waits for
  it (≤ 10 s), the client asks for the delta once the original has loaded — the
  cloud is gone on the way back to the list. A medium or a hover refreshes in the
  background. The download next to Photos' original is gone: it is a JPEG every
  browser shows — the download stays only as the fallback for a local original the
  browser cannot show.
- **Viewer switches** (the owner): a Live Photo's motion has one button with three
  states — off → once → loop (`viewerPrefs.liveMode`; the old switch carries over:
  off stays off, on is once). The wheel zoom is gone altogether (the owner: it
  worked badly — Immich and Google do it better, each differently): roadmap.
- **Hover UX** (the owner, as Immich and Google Photos): the badge stays while the
  tile moves, a ring turns around its mark while the video comes (shown only after
  300 ms: a video that starts at once made it blink), the video fades in
  over the frames (200 ms).
- **A local HEIC original leaves no file**: asked for the image, Photos draws it
  from the original on disk and writes no derivative — the viewer got a 404. The
  image PhotoKit hands over comes back as JPEG (`pk_image`) and is served when no
  file appeared (not kept; the browser caches it). A failed request serves nothing.
- **In the server** (PR #21): the request does not reprocess anything — the
  endpoint serves the file from the library right after Photos made it local, and
  the next walk adds it to the group (a new file), so the item and the other tabs get
  it through the delta. The video renditions go after the stills in the group: a
  cloud-only video's main file stays its still. Asking for access is done only when
  a `*.photoslibrary` is under the root (no prompt for a folder library). Not
  checked in the agent's shell (no Photos access there); the user's server is the
  test.
- Traps: asynchronous PhotoKit results are delivered on the main queue, which a
  command-line tool does not run — the request never came back (synchronous requests
  from a cgo thread work); yet Photos finished the download it had started. In the
  server: images synchronous, videos come on any queue, Live Photos on the main queue
  — `photokit` locks main to the main thread (`LockOSThread` in `init`) and
  `RunMain` turns its run loop in place of waiting for a stop. The `.m` file is
  `photokit_darwin.m`: without the suffix a Linux build (no cgo in the package)
  refuses it. `NSImage`
  sizes are points (×2 on Retina): the bitmap's pixels come from its `CGImage`.

### Apple Photos library: spike (2026-09-30)

On a copy of the dev library's `Photos.sqlite` (read with `mode=ro`) and a list of
the bundle's files (`originals/`, `resources/renders/`, `resources/derivatives/`).
Every file belongs to a known asset; the DB's local-resource counts match the files.

- 6 457 assets: 5 691 photos (367 Live Photos, 449 screenshots, 22 panoramas), 766
  videos; 30 trashed, none hidden. Schema `ZASSET` (macOS 11+).
- The DB stores only the original's path: `originals/<ZDIRECTORY>/<ZFILENAME>`.
  Everything else follows a naming layout (`<X>` = the UUID's first character):

  | File | What | Long side |
  |---|---|---|
  | `originals/<X>/<UUID>.<ext>` | the original (source) | full |
  | `resources/renders/<X>/<UUID>_1_201_a.jpeg\|heic` | the user's edit, full size | ~1600, up to 5700 |
  | `resources/renders/<X>/<UUID>.plist` | edit data (not an image) | — |
  | `resources/derivatives/<X>/<UUID>_1_101_o`, `_1_102_o.jpeg` | preview of the original | ~2000–2600 |
  | `resources/derivatives/<X>/<UUID>_1_102_a.jpeg` | preview of the edit | ~2000 |
  | `resources/derivatives/<X>/<UUID>_1_105_c`, `_1_106_c.jpeg` | medium preview | ~1000 |
  | `resources/derivatives/masters/<X>/<UUID>_4_5005_c.jpeg` | small thumbnail, nearly every asset | ~640 (360×640 measured; not in the DB) |
  | `resources/derivatives/<X>/<UUID>.THM` | video "poster": a JPEG icon | 32×32 (measured) |
  | `resources/derivatives/cvt/<X>/<UUID>/…_cvt_tNNNN.jpeg` | video frames (scrubbing) | — |

  In `ZINTERNALRESOURCE` (local rows): `(type 0, version 0, subtype 1)` = originals,
  `(0,2,2)` = renders, `(0,0,4)` = `_1_102_o`, `(0,0,3)` = `_1_101_o`, `(0,3,0)` =
  `_1_105_c`, `(0,2,4)` = `_1_102_a`, `(14,3,0)` = masters; video = type 1, Live
  Photo video = type 3.
- **Optimize Mac Storage**: 1 782 originals are local, none of the videos and none of
  the Live Photo videos. Best local preview per live photo asset (5 661): original
  1 781; render 328; ~2000 px derivative 1 463; only the small master 2 082; nothing
  7. Videos (766): poster `.THM` 544, a ~2000 px image 219, other 3. **Nothing at all:
  7 of 6 427**. So a
  derivative can show 6 420 assets where the originals alone show 1 781.
- Decisions: cloud-only assets are items (preview from the derivative, metadata from
  the DB); an edited photo shows its edit; any size counts; Apple's derivatives are
  used as they are — transcode only fills gaps (see roadmap P3).
- Unverified here: the Live Photo video's file name (`<UUID>_3.mov` per osxphotos) —
  no Live Photo video is local in this library.

### Item == asset (2026-09-29)

- **Decision: one entity.** An item is the asset — one whole group of files (source
  + sidecars + derivatives); no separate asset/item split. What must hold instead:
  a derivative never becomes an item of its own, it is linked to its asset.
- Found: 65 items from an Apple Photos library were not assets — images of Apple's
  own in `internal/` (Messages backdrops) and `scopes/` (iCloud sharing); only the
  `resources/` exclusion kept derivatives out. Now only `originals/` of a library
  are read (until the Apple Photos grouper exists); the rest of the bundle is
  skipped, so the old items are soft-deleted by the next complete walk (not seen =
  gone).
- A derivative is simply a file linked to its source (`LinkedTo`) — no separate
  "derivative" flag. For the Apple Photos grouper the links come from
  `Photos.sqlite` (original = source; render, derivatives, Live Photo video =
  linked to it): the grouper states the main file and mime does not re-rank such a
  group — it ranks only groups nobody decided (generic). Otherwise an original HEIC
  and its render JPEG, both images, would be decided by size.

### Import chain as small steps (2026-09-29)

- **Groupers are plain decorators with a buffer of open groups** (decision): a group
  goes out when it is complete, so each incoming file closes at most one group —
  one output per call is enough. A first version buffered a whole directory
  (`WalkDir` visits subdirectories between a directory's files) and released many
  groups at once, which needed a 1 → N step in perceplib (`Expander`); dropped:
  sidecars are next to their main file, one open group suffices for `generic`.
  Known limit: a name sorting between members splits a group (`a.aae`,
  `a.edited.jpg`, `a.jpg`; a subdirectory `a.jpg.d/` between `a.jpg` and `a.xmp`).
  Skipped items (`ErrSkippedItem`) are not logged any more.
- **exif was serial.** A decorator runner is one goroutine: the pool of 5 exiftool
  processes was used one at a time. Now N runners read the same channel (groups are
  independent after the gate); `Stop` is called by each, closing is `sync.Once`.
- **The main file is known only after exif**, so every file of a group gets `-all`
  (same arguments as the main file had: short hashes stay stable) and the validator
  links the group. **The main file is always the source** (decision): RAW > video >
  image. The JPEG of RAW+JPEG and the photo of a Live Photo are derivatives —
  sidecars that can later serve as ready previews. A file that was a main file and
  becomes a sidecar (a JPEG imported before its RAW) loses its item. Side effect:
  tags missing in the main file can now come from a sidecar's full set (`GetExif`
  looks through the group, main first).
- **The closer blocked after 1000 items**: it wrote to a buffered channel nobody
  read. Finished items are drained now (later: events to the client).
- Codex review: (1) a derivative whose main file was deleted stayed linked to the
  deleted item and was dropped by the gate — now a link to a GUID outside the group
  means "process"; (2) groups ignored by the old system-MIME logic would stay
  ignored forever — a `meta` table keeps `mime_version`, a new version clears the
  "ignored" marks once.
- The gate stamps `CheckTime` with its own clock; the marker carries the walk start,
  and "not stamped since the walk started" = gone.

### Go plugins (2026-09-28)

- Host and `.so` must be built with **the same toolchain** and **identical versions
  of shared packages** (`perceplib`, `zap`, `multierr`). So the Go version and
  dependencies are bumped together in `perceplib`, `gontroller` and every
  `perceptors/*`, then `make build-plugins`.
- `TestLoadExternalPlugins` does not actually load any `.so` (the mock points to
  `test_plugin.so`) — it passes with a warning. Real loading was checked with a
  temporary test via `loadPlugin("../../.build/plugins/<name>.so")`: `exif_geo`,
  `ml_color` — ok; `ml_faces`, `ml_objects` — stubs without a `Perceptor` symbol.
- **Pipeline stall with external EXIF plugins (fixed 2026-09-29):**
  `ExifPluginProcessor` allocated a channel for every `ExifDataProvider` plugin but
  added a step only for `exif_core.ExifCorePerceptor` (`RawItemRW`). External
  `exif_geo` implements `api.ExifPerceptor` (`RawItemR`) → nobody read its channel →
  no item reached the closer. Confirmed with an end-to-end test (0 of 2 items).
  Now external plugins are wired through two adapter decorators
  (`RawItemRW → RawItemR → RawItemRW`), and a channel is allocated only for a step
  that exists.

### exiftool → own package (2026-09-28)

- The wrapper is a fork of `ncruces/go-exiftool` (upstream unmaintained for years;
  the channeled-output PR was declined). It is now `github.com/eggs-gd/go-exiftool`
  (v0.5.0); the local clone lives next to Perceptrail, not inside it.
- **A clone with its own `go.mod` inside the gontroller module becomes a separate
  module** → the `perceptrail/gontroller/lib/exiftool` import breaks. Hence a tagged
  package.
- `gh pr create` in a fork targets upstream by default → always
  `--repo eggs-gd/go-exiftool --base main`.
- Actions are disabled in forks until enabled; after the transfer the workflows got
  registered.
- **ExifTool distribution:** `exiftool.org` no longer serves tarballs (404).
  SourceForge keeps only the last few versions, CPAN keeps production releases
  forever. Not every version on the site is "production" (2026-09: production 13.55
  while 13.59 is already out). Use the latest production release.
- The version lives in one place — `EXIFTOOL_VERSION`. A `dist-<version>` tag → CI
  builds, runs `prove` and publishes a GitHub Release (`exiftool_unix.tgz`,
  `exiftool_windows.zip`, `SHA256SUMS`). `dist-*` is not semver → no clash with `v*`
  module tags. The Perceptrail Docker image pulls exactly this release.
- Wrapper weak spots fixed in v0.5.1 (2026-09-29): per-command timeout (`SetTimeout`,
  gontroller uses 2 min) with restart; `Wait` after `Kill` (no zombies); the `start()`
  error of a restart is returned; `Command` returns stdout together with a stderr
  error (it used to drop it — gontroller then stored an empty `RawExif`).
- Extractor rule: no usable EXIF for the **main** file → the group is skipped (it
  used to be validated against a sidecar's EXIF, or panic on an empty list); a
  sidecar without EXIF is just dropped; data + warning → data is used.

### Data and import

### Dates and time zones (2026-09-29)

- **Most items had no date:** the `FileModifyDate` fallback never parsed — exiftool
  prints it with a zone (`…+03:00`). Files without EXIF (screenshots, messenger
  images) are the majority of a real library. Fixed; on the dev library (outside the
  Photos bundle) 0 zero dates, 545 of 570 dated by `FileModifyDate`.
- **The zone is stored separately** (`DateOffset`, minutes): sqlite and Postgres
  `timestamptz` return times in UTC, the zone of `Date` is lost on read.
  `RawItem.GetDate()` returns the date in the zone of the shot, so external plugins
  see it through the unchanged perceplib API; only the core `exif_core.RawItemRW`
  got `SetDateInfo` — no perceplib release, no plugin rebuild.
- **No `-G` in exiftool:** groups would rename every tag (plugins read `ImageWidth`,
  the short hash hashes tag names). `CreateDate` is interpreted by file type instead:
  EXIF local time for images, QuickTime UTC for videos (checked on a real `.MOV`:
  `CreateDate 11:29:03` vs `CreationDate 14:29:03+03:00`).
- **Zone chain:** the tag's offset → local time − GPS time rounded to 15 min (GPS
  fixes lag seconds) → coordinates → IANA zone via `github.com/ringsaturn/tzf`
  (boundaries embedded, ~12 MB of binary, 0.17 s to load, lazily) → the server's zone
  (assumed). Not longitude/15: no DST, wrong at administrative borders. The time zone
  is core (the date), not the geo perceptor. `time/tzdata` is embedded for Docker.

### Validator per `Walker.puml` (2026-09-29)

- **Walk safety first.** One unreadable directory aborted the whole walk (the
  `WalkDir` callback returned the error). Now it is skipped and recorded; its files
  are never taken as deleted. Deletions run only after a complete walk that found
  files: a cancel, a missing root or an empty mount point would otherwise delete the
  library. Real case: without Full Disk Access the Photos library is unreadable.
- **Finalization runs in `Decorate` on the end-of-walk marker**, after the last group
  is stored — `Start` (walk) and `Decorate` (DB writes) are different goroutines. It
  was in `Stop()`, called by both goroutines and on cancel.
- **Moves race with deletions.** Validation happens downstream (exiftool workers,
  channels), so finalization may delete the vanished path's item before the moved
  file is validated. Fix: move detection also looks at soft-deleted items with the
  same hash whose path is gone, and restores them (as `Dirty`). Order-independent,
  and a file that comes back later gets its old GUID.
- **States are alive:** the closer sets `Ready`; the walker re-emits unchanged groups
  whose item is not `Ready`. The first run after this change reprocesses everything
  once (all items were `New`).
- CheckTime is compared in Go: the driver stores times as text with the local
  offset, which changes across DST — a SQL string comparison is not reliable.
- Tests without exiftool: the file content stands in for metadata
  (`pkg/scan/validator_test.go`), one sqlite per package (`TestMain`).


- **fswalker (fixed 2026-09-29, found by Codex review):** the last group of a walk was
  never emitted (groups go out when the next group starts) — a library with a single
  group imported nothing; now an end-of-walk marker flushes it. For changed files the
  stale DB row (old size/mtime) was saved back — every scan saw them as changed and
  `HashShort` used the old size; now fresh values are stored. `dbitems[i]` indexing
  drifted after a `continue` (possible out-of-range).

- **`HashShort` was unstable:** it hashed a pointer address (`&item.Size`), and after
  `--File:all` was dropped also volatile File-group tags (`FileAccessDate`,
  `FileName`, `Directory`). Fixed: size + EXIF without volatile tags. This is what
  makes the "found moved" branch of the validator possible — see
  [`Walker.puml`](../puml/Walker.puml) (same / moved / duplicate / changed).
- **Video size:** QuickTime stores unrotated dimensions + `Rotation` (90/270) — handled
  in `size` next to EXIF `Orientation` (5–8).
- **MIME detection in fswalker is platform-dependent:** `mime.TypeByExtension` uses
  system tables; in a minimal Docker image `.mov/.heic/RAW` → `application/octet-stream`
  → the whole group is ignored. Needs an own media-type table (`MediaKind`).
- Grouping: the main file of a Live Photo is currently the **video** (the comparator
  ranks `video/` above `image/`); for RAW+JPEG the main file is nondeterministic.
- `date`: the `FileModifyDate` fallback never parses (exiftool prints it with a zone,
  `…+02:00`); no time zone handling at all (`OffsetTimeOriginal` is not read).
- SQLite: WAL + `busy_timeout` in the DSN, **a single connection** (otherwise
  "database is locked"). Consequence: never query through `db` instead of `tx` inside
  a transaction — deadlock. `/items` streams NDJSON in keyset pages of 32.

---

## Infrastructure

### `app.css` was never loaded (2026-09-29)

- No module imported `src/app.css`, so none of its rules ever applied: the page was
  always white and `scrollbar-gutter: stable` (added against the width jump when the
  viewer opens) never worked. It surfaced when the viewer switched from a hard-coded
  `#111` to `var(--color-bg)` from `app.css` and lost its background. Now the root
  `+layout.svelte` imports it. Check that a global stylesheet is actually loaded
  before relying on its variables.

### Safari: 500 on every page (2026-09-29)

- Safari showed SvelteKit's 500 page with `ReferenceError: Cannot access 'load' before
  initialization`. Not a circular import: `getLogger()` read the caller from
  `stack.split('\n')[2]` — Chrome's format (`Error` header + `at …` lines). Safari and
  Firefox print `fn@url:line:col` without a header, so at module top level there was
  no third line → `undefined.match` threw while `proxy.ts` initialised → `+layout.ts`
  never finished → Kit's export validator (`for…in` over the module namespace, which
  JavaScriptCore evaluates, V8 does not) hit the uninitialised `load`.
- `callerContext()` now parses both formats and never throws. Rule: code that runs at
  module initialisation must not throw — in Safari it takes the whole app down.

### perceplib: subtree instead of submodule (2026-09-29)

- The submodule kept biting: detached HEAD in the checkout; `gontroller` depended on
  uncommitted `perceplib` changes (a fresh clone would not build); every change was
  two commits (library + pointer); clones need `--recurse-submodules`.
- `perceplib` must stay a separate **public** repo with tags (third-party perceptors
  `go get` it) while Perceptrail is private — so no monorepo yet.
- Decision: `git subtree` under `perceplib/`. Everyday work is one commit in
  Perceptrail; publishing is explicit: `git subtree push` + a `vX.Y.Z` tag in
  `eggs-gd/perceplib` (procedure in `perceplib/README.md`).
- Later, once Perceptrail is public: consider a monorepo with `perceplib` as a
  subdirectory module (`github.com/eggs-gd/perceptrail/perceplib`, tags
  `perceplib/vX.Y.Z`) and archive the separate repo.

- **Versions (2026-09-29):** Go 1.27.1 in all modules; svebapp — Vite 8, Svelte 5.57,
  Kit 2.70, TypeScript 6 (TS 7 is not supported by Kit and svelte-check yet).
- `npm` refused to upgrade because of a stale lock (`vite-plugin-svelte-inspector@3`
  pinned the plugin to v4) → the lock was regenerated from scratch, no `--force`.
- `npm audit`: `cookie@0.6.0` comes from Kit itself; the only "fix" downgrades Kit —
  left as is.
- gopls built with an older Go cannot read code for a newer one — after a Go bump:
  `GOTOOLCHAIN=go1.27.1 go install golang.org/x/tools/gopls@latest`.
- ImageMagick without rsvg drops SVG strokes → rasterise with `resvg`
  (`@resvg/resvg-js`).
- Favicon "trail of frames" — a metaphor for navigation by similarity: looking for an
  unnoticed trail through the photos.
