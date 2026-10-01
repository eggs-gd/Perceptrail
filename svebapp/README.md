# svebapp

The Perceptrail client: a SvelteKit gallery (Svelte 5, Vite 8), data in IndexedDB
(Dexie), loading and layout in Web Workers.

Design: [Workers](../_sb/puml/Workers.puml), [Protocol](../_sb/puml/Protocol.puml).

## Running

```bash
npm install
npm run dev       # dev server
npm run check     # svelte-check (must be 0 errors)
npm run build     # production build (adapter-node)
```

`.env`: `PUBLIC_API_PATH` — gontroller address (`http://localhost:1323` in the repo).
Variables prefixed with `SVEBAPP_` are read by adapter-node.

## Architecture

The original design of [Workers.puml](../_sb/puml/Workers.puml): workers write to
IndexedDB, the page subscribes with `liveQuery` — no worker → UI messages.

```
gontroller /items (NDJSON, one item at a time)
   │
   ▼
wsync (worker) ── put ──► itemsDb (Dexie "items")
   │  Dexie hooks: create / update / delete, sync-start / sync-done
   ▼  MessageChannel (worker → worker)
wlayout (worker)   all photos in memory, in the active navigator's order
   │  (GET /p/:name/order: guids + sections), synchronous layout → x, y, w, h
   │  writes layoutDb: a full relayout = ONE transaction (items + meta + sections),
   │  a streamed photo = one small transaction
   ▼
layoutDb (Dexie "layout": items by y/order, meta {rev, height, anchor}, sections)
   │  liveQuery over the VISIBLE WINDOW only (layoutWindow.ts)
   ▼
Gallery.svelte     renders the window: absolutely positioned tiles, keyed by guid;
                   GalleryTools (navigator buttons, panel pin), SidePanel (sections)
```

Navigators ([Perceptors.puml](../_sb/puml/Perceptors.puml)): `GET /perceptors` gives
a button per navigator (`lib/gallery/perceptors.svelte.ts` keeps the active one and
the pinned panel per browser). A switch is carried out like a resize: the photo in
the middle of the screen (or the viewer's photo, centred) is the anchor the relayout
keeps in place.

- `lib/workers/proxy.ts` — creates the workers, `loadFromServer()`,
  `updateLayout(screenWidth, rowHeight, anchor?)`.
- `lib/workers/tasks/wsync.ts` — clears `itemsDb`, reads the NDJSON stream, writes items.
- `lib/workers/tasks/wlayout.ts` — row layout (greedy: fits → into the row;
  overflow < ½ of the photo → close the row without it; otherwise add it and shrink
  the row; the last row stays at the target height). All layoutDb writes go through
  one ordered queue; at most one full relayout waits in it; streamed photos are
  batched per ~16 ms.
- `lib/gallery/layoutWindow.ts` — `watchWindow()`: one `liveQuery` that reads the
  `layout` record and the window's items in one read transaction (a consistent
  snapshot); `watchSize()`; `findAnchor()`.
- `lib/gallery/Gallery.svelte` — tracks scroll, subscribes to the window, applies at
  most one snapshot per frame, captures the anchor on resize, the "wave" animation.
- `routes/[index=itemIndex]` — the viewer reads its item from `layoutDb` by `order`.
- `lib/workers/layout/` — **unused**: the original optimal masonry layout with
  Dijkstra. `dijkstra.js` is third-party MIT code under `@ts-nocheck`.

### Visible window and resize anchoring

- Window = `y` range `[scroll − 1 viewport, scroll + 2 viewports]` (layout
  coordinates), moving in steps of half a viewport — scrolling inside a step does not
  re-create the subscription. The query finds the first row reaching into the window
  by the indexed `bottom` (rows can be taller than the window), then takes the rows
  starting before its end. All ranges are bounded: writes outside the window do not
  re-run it.
- Tiles keep `loading="lazy"`: the gallery still loads **originals**, and lazy
  loading limits how many are decoded at once (without it blank tiles get more
  frequent). `overflow-anchor: none` — we anchor the view ourselves.
- On a width change the gallery takes an **anchor** — what stays pinned on screen:
  at the very top of the page the page stays at the top; at the very bottom the end
  of the gallery stays at the bottom; anywhere else the photo under the middle of the
  screen stays there (`guid` + offset as a fraction of its height). It is sent with
  `updateLayout` and kept across all following resizes until the user scrolls
  (a scroll that is not our own correction, detected in `onscroll`).
- **FLIP per tile (Web Animations):** before a new layout is applied, every tile's
  current box is measured (mid-animation included); after the DOM update and the
  scroll correction (Δ) each tile animates from its current screen position
  (shifted by Δ, so nothing jumps) to its new place. No CSS transitions on tiles.
- **Wave from the anchor, by rows:** rows start by their distance from the anchor's
  row (rows at the same distance above and below start together); within a row photos
  go left to right, in the anchor's row from the anchor outwards. Page top: from the
  first visible row down; page bottom: from the last visible row up. A tile waiting for
  its turn stays exactly where it was (`fill: backwards`). The wave over the photos
  visible before or after fits into `STAGGER_MAX_MS`. Off with `prefers-reduced-motion`.
- Tiles mounted by a relayout appear in place at once (no fade): widening brings in
  many photos that were not rendered, and fading them from 0 flashed the screen white.
  Photos arriving with the stream still fade in.
  The worker writes the relayout and `meta.anchor` (the anchor's new `y`, `h`) in one
  transaction. The window query sees a new `meta.rev` with that anchor, takes the
  window around the new position and returns `scrollTo`; the gallery applies height
  and items first, then scrolls (`tick()` — the browser clamps scrolling to the
  current document height).
- `meta` has two records: `layout` (rev, width, anchor — only relayouts write it; the
  window query reads it) and `size` (height, count — every streamed batch; a separate
  subscription). Never let the window query read something that changes per photo,
  and never subscribe to the whole table.
- Streamed photos are written in batches (one transaction per ~16 ms).

### Worker messages (`MessageFromSync`, wsync → wlayout)

| action | Payload |
|---|---|
| `sync-start`, `sync-done` | — |
| `create`, `update`, `delete` | `item` |

## Conventions (Svelte 5 / Kit 2)

Checked against the Svelte docs (Svelte MCP `get-documentation`: best practices).

- `$app/state`, not `$app/stores`; no `svelte/store` — component state with runes,
  external data via `LiveQuery` (`lib/stores/internal/liveQuery.ts`, a Dexie
  `liveQuery` on `createSubscriber`; create it in `$derived` when it depends on state).
  `LiveQuery` is not re-exported from `$lib/stores` (workers import that module).
- `$derived` for anything computed; `$effect` only for real integrations (worker
  messages, the window subscription with its per-frame scheduling and scroll
  correction, `document.body` style).
- `{@attach}` instead of actions / `onMount` + `bind:this` for element listeners.
- `class={[...]}` / `class={{...}}` instead of `class:`; `style:` for single styles.
- No side effects in `load` (it runs on the server and on every navigation): the sync
  starts in `onMount` of the root layout.
- Code that runs at module initialisation must never throw (Safari takes the whole app
  down, see findings).

## Rules learned the hard way

Details and reasons — [findings](../_sb/docs/findings.md#frontend-gallery-layout-resize-streaming).

- Compute the layout **synchronously, in memory** in the worker. No async tasks with
  cancellation and no per-task hook subscriptions — that caused races and "vanished"
  photos.
- A relayout is **one transaction** (items + meta), without clearing the view first.
- The page renders the visible window only and applies at most one snapshot per
  frame. Otherwise the main thread is busy and clicks and resize do not work.
- No `.clear()` on module import (several contexts import the Dexie modules).
- Tiles are absolute, keyed by `guid`, moved with CSS `transition` on `transform`,
  `width`, `height`. Not `animate:flip` (measures every tile) and not one `{#each}`
  per row (a tile cannot move between rows).
- While dragging the window edge a new layout arrives every ~33 ms; any animation
  delays must stay short (≤ 150 ms now).
- Test in a **visible, painting** tab: in a hidden one `requestAnimationFrame`,
  `ResizeObserver` and scroll events stop (the gallery falls back to `setTimeout` for
  snapshots, but a resize or scroll is never noticed). Streaming issues need a slow
  backend (a mock with a delay).
- `GUTTER` in `wlayout.ts` must match `gutter` in `Gallery.svelte`.
