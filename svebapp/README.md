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

```
gontroller /items (NDJSON, one item at a time)
   │
   ▼
wsync (worker) ── put ──► itemsDb (Dexie "items")
   │  Dexie hooks: create / update / delete, sync-start / sync-done
   ▼  MessageChannel
wlayout (worker)   all photos in memory, synchronous layout → x, y, w, h
   │  upsert (each new photo) / replace (full rebuild, e.g. resize)
   ▼  MessageChannel
stores.ts          queue → applied once per frame (requestAnimationFrame)
   │  layoutItems (Map) → items (sorted)
   ▼
Gallery.svelte     flat list of absolutely positioned tiles, keyed by guid
```

This deviates from the original design in [Workers.puml](../_sb/puml/Workers.puml)
(LayoutWorker writes to `LayoutDB`, the View subscribes to it) — see "liveQuery"
below.

- `lib/workers/proxy.ts` — creates the workers and channels, `loadFromServer()`,
  `updateLayout(screenWidth, rowHeight)`.
- `lib/workers/tasks/wsync.ts` — reads the NDJSON stream and writes to `itemsDb`.
- `lib/workers/tasks/wlayout.ts` — row layout (greedy: fits → into the row;
  overflow < ½ of the photo → close the row without it; otherwise add it and shrink
  the row). The last, incomplete row stays at the target height.
- `lib/stores` — Svelte stores, types (`Item`, `LayoutItem`), Dexie databases.
- `lib/gallery` — `Gallery.svelte`, components `ItemView/Img/Video`.
- `routes/+layout.svelte` — the gallery lives here so it is not remounted on
  `/` ↔ `/[index]`; `routes/[index=itemIndex]` — the viewer with zoom.
- `lib/workers/layout/` — **unused**: the original optimal masonry layout with
  Dijkstra (a graph of row breaks). Kept for a possible relayout of an already loaded
  gallery; `dijkstra.js` is third-party MIT code under `@ts-nocheck`.
- `lib/stores/internal/layoutDb.ts` — unused right now; the base for the liveQuery
  design.

### Message protocol (`MessageFromSync`)

| From → to | action | Payload |
|---|---|---|
| wsync → wlayout | `sync-start`, `sync-done` | — |
| wsync → wlayout | `create`, `update`, `delete` | `item` |
| wlayout → view | `upsert` | `items` — new/changed positions |
| wlayout → view | `replace` | `items` — the whole layout (replaces the view) |

## Rules learned the hard way

Details and reasons — [findings](../_sb/docs/findings.md#frontend-gallery-layout-resize-streaming).

- Compute the layout **synchronously, in memory** in the worker. No async tasks with
  cancellation and no per-task hook subscriptions — that caused races and "vanished"
  photos.
- A resize is **one `replace`**, without clearing the view first.
- The UI does not process messages one by one: queue + once per frame. Otherwise the
  main thread is busy and clicks and resize do not work.
- Tiles are absolute, keyed by `guid`, moved with CSS `transition` on `transform`,
  `width`, `height`. Not `animate:flip` (measures every tile) and not one `{#each}`
  per row (a tile cannot move between rows).
- While dragging the window edge a new layout arrives every ~33 ms; any animation
  delays must stay short (≤ 150 ms now).
- Test animations in a **visible** tab only: `requestAnimationFrame` does not fire in
  a hidden one. Streaming issues need a slow backend (a mock with a delay).
- `GUTTER` in `wlayout.ts` must match `gutter` in `Gallery.svelte`.

## Next: liveQuery

Back to the original design: workers write to IndexedDB directly, the UI holds
`liveQuery` subscriptions to the **visible window** (`where('order').between(from, to)`)
— no worker ↔ UI messaging, and virtualisation for free. First, an experiment: do
events cross threads on Dexie 4. Details and caveats — in findings.
