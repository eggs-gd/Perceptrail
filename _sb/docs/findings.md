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
  - Not a bug: with the mock's repeating aspect ratios a 1008 px layout is taller per
    photo than a 700 px one (rows of 3 stretched to the full width), so the page gets
    shorter when narrowed mid-stream.
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

## Backend: gontroller, plugins, exiftool

### Go plugins (2026-09-28)

- Host and `.so` must be built with **the same toolchain** and **identical versions
  of shared packages** (`perceplib`, `zap`, `multierr`). So the Go version and
  dependencies are bumped together in `perceplib`, `gontroller` and every
  `perceptors/*`, then `make build-plugins`.
- `TestLoadExternalPlugins` does not actually load any `.so` (the mock points to
  `test_plugin.so`) — it passes with a warning. Real loading was checked with a
  temporary test via `loadPlugin("../../build/plugins/<name>.so")`: `exif_geo`,
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
