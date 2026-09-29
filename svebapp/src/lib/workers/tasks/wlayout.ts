import {
    type MessageFromSync,
    type UpdateLayoutPayload,
    type WorkerMessage,
} from "./types";
import {
    type Item,
    LAYOUT_META_KEY,
    LAYOUT_SIZE_KEY,
    layoutDb,
    type LayoutItem,
    type LayoutMeta,
    type LayoutSize,
} from "$lib/stores";
import {getLogger} from "$lib/logger";

const logger = getLogger()

// Must match Gallery.svelte gutter (gap between items and rows)
const GUTTER = 8;
// Window-edge drag sends a resize per frame; lay out at most this often (~30 fps)
const RELAYOUT_THROTTLE_MS = 33;
// Streamed photos are written in batches: one transaction per frame, not per photo
const APPEND_FLUSH_MS = 16;

let viewport: UpdateLayoutPayload | null = null;
let relayoutTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Every item streamed so far, in stream (id) order. Kept in memory so a relayout
 * is one synchronous computation; the result goes to layoutDb, where the page
 * reads the visible window with liveQuery (Workers.puml).
 */
let items: Item[] = [];

/** Incremental layout state for the current viewport */
interface LayoutState {
    /** Items of the row that is still being filled (not stretched yet) */
    openRow: LayoutItem[],
    rowNum: number,
    /** Top of the open row, px */
    y: number,
}

let state: LayoutState = freshState();
let meta: LayoutMeta = {key: LAYOUT_META_KEY, rev: 0, width: 0, height: 0};

/** Streamed rows waiting for the next batch write (latest position per guid) */
let pendingRows = new Map<string, LayoutItem>();
let flushTimer: ReturnType<typeof setTimeout> | null = null;

function freshState(): LayoutState {
    return {openRow: [], rowNum: 0, y: 0};
}

/**
 * All layoutDb writes go through one queue, in order. A full relayout is computed
 * when its turn comes (latest viewport and items), and at most one is waiting:
 * during a window-edge drag the writes cannot pile up.
 */
let writes: Promise<unknown> = Promise.resolve();
let relayoutQueued = false;

function enqueue(write: () => Promise<unknown>) {
    writes = writes.then(write).catch((error) => logger.error('layout write failed', error));
}

self.onmessage = function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    if (task === 'init') {
        const itemsDbPort: MessagePort = payload[0];
        itemsDbPort.onmessage = onItemsDbMessage;
        logger.debug('Inited')
    } else if (task === 'update') {
        const p: UpdateLayoutPayload = payload;
        if (viewport?.screenWidth === p.screenWidth && viewport?.rowHeight === p.rowHeight) {
            return;
        }
        viewport = {screenWidth: p.screenWidth, rowHeight: p.rowHeight, anchor: p.anchor};
        scheduleRelayout();
    }
};

function onItemsDbMessage(event: MessageEvent<MessageFromSync>) {
    const {action, item} = event.data;

    switch (action) {
        case 'sync-start':
            items = [];
            queueRelayout();
            return;
        case 'create': {
            const last = items[items.length - 1];
            if (!last || item!.id > last.id) {
                items.push(item!);
                append(item!);
            } else {
                // Out of stream order: put it in place and lay out everything again
                insertSorted(item!);
                scheduleRelayout();
            }
            return;
        }
        case 'update': {
            // An already placed item changed size: everything after it moves
            const i = items.findIndex((itm) => itm.guid === item!.guid);
            if (i >= 0) items[i] = item!;
            scheduleRelayout();
            return;
        }
        case 'delete':
            items = items.filter((itm) => itm.guid !== item!.guid);
            scheduleRelayout();
            return;
    }
}

function insertSorted(item: Item) {
    const i = items.findIndex((itm) => itm.id > item.id);
    if (i < 0) items.push(item);
    else items.splice(i, 0, item);
}

function scheduleRelayout() {
    if (relayoutTimer) return; // the pending run uses the latest viewport and items
    relayoutTimer = setTimeout(() => {
        relayoutTimer = null;
        queueRelayout();
    }, RELAYOUT_THROTTLE_MS);
}

function queueRelayout() {
    if (relayoutQueued) return;
    relayoutQueued = true;
    // Rows computed for the old layout must not land after the relayout
    dropPendingRows();
    enqueue(() => {
        relayoutQueued = false;
        return writeRelayout();
    });
}

/**
 * Lays out every item from scratch and replaces the layout in ONE transaction,
 * together with meta (height, rev, new position of the anchor item): the page
 * gets one consistent snapshot per relayout.
 */
function writeRelayout(): Promise<unknown> {
    state = freshState();
    if (!viewport) return Promise.resolve();
    const vp = viewport;

    const all: LayoutItem[] = [];
    for (let i = 0; i < items.length; i++) {
        all.push(...place(items[i], i, vp));
    }
    all.push(...placeOpenRow(vp));

    const anchor = vp.anchor ? all.find((itm) => itm.guid === vp.anchor) : undefined;
    meta = {
        key: LAYOUT_META_KEY,
        rev: meta.rev + 1,
        width: vp.screenWidth,
        height: totalHeight(vp),
        anchor: anchor && {guid: anchor.guid, y: anchor.y, h: anchor.h},
    };
    // Copies: the open row keeps changing after this, the write must not see that
    const rows = all.map((itm) => ({...itm}));
    const layoutRecord = {...meta};
    const sizeRecord = currentSize(vp);

    logger.debug('relayout', {items: rows.length, width: vp.screenWidth});
    return layoutDb.transaction('rw', layoutDb.items, layoutDb.meta, async () => {
        await layoutDb.items.clear();
        await layoutDb.items.bulkPut(rows);
        await layoutDb.meta.bulkPut([layoutRecord, sizeRecord]);
    });
}

/**
 * Places one streamed item. The changed rows are collected and written once per
 * APPEND_FLUSH_MS: photos still appear one by one (per frame), but IndexedDB gets one
 * transaction per batch, and only the 'size' record changes — the window query
 * (which reads 'layout') re-runs only if new photos land inside the window.
 */
function append(item: Item) {
    // No viewport yet or a full relayout is pending: that run will include this item
    if (!viewport || relayoutTimer || relayoutQueued) return;
    const vp = viewport;

    const changed = place(item, items.length - 1, vp);
    changed.push(...placeOpenRow(vp));
    for (const itm of changed) {
        pendingRows.set(itm.guid, {...itm}); // copy: the open row keeps changing
    }
    if (!flushTimer) {
        flushTimer = setTimeout(flushAppends, APPEND_FLUSH_MS);
    }
}

function flushAppends() {
    flushTimer = null;
    if (!viewport || pendingRows.size === 0) return;

    const rows = [...pendingRows.values()];
    pendingRows = new Map();
    const sizeRecord = currentSize(viewport);
    enqueue(() => layoutDb.transaction('rw', layoutDb.items, layoutDb.meta, async () => {
        await layoutDb.items.bulkPut(rows);
        await layoutDb.meta.put(sizeRecord);
    }));
}

function dropPendingRows() {
    pendingRows = new Map();
    if (flushTimer) {
        clearTimeout(flushTimer);
        flushTimer = null;
    }
}

function currentSize(vp: UpdateLayoutPayload): LayoutSize {
    return {key: LAYOUT_SIZE_KEY, rev: meta.rev, height: totalHeight(vp), count: items.length};
}

/** Gallery height: closed rows (each followed by a gutter) plus the open row */
function totalHeight(vp: UpdateLayoutPayload): number {
    return state.openRow.length > 0 ? state.y + vp.rowHeight : Math.max(0, state.y - GUTTER);
}

function gapsForRow(itemCount: number): number {
    return Math.max(0, itemCount - 1) * GUTTER;
}

function naturalWidth(itm: Item, rowHeight: number): number {
    return itm.width * rowHeight / itm.height;
}

/**
 * Adds an item to the open row. Returns the items of a row that got closed
 * (stretched to the full width, final positions) — or nothing.
 */
function place(item: Item, order: number, vp: UpdateLayoutPayload): LayoutItem[] {
    const lItem: LayoutItem = {
        ...item,
        order,
        row: state.rowNum,
        scale: 1, x: 0, y: 0, w: 0, h: 0,
    };

    const row = state.openRow;
    const rowWidth = row.reduce((sum, itm) => sum + naturalWidth(itm, vp.rowHeight), 0);
    const newItemWidth = naturalWidth(lItem, vp.rowHeight);
    const deltaWidth = vp.screenWidth - (rowWidth + newItemWidth + gapsForRow(row.length + 1));

    if (deltaWidth >= 0) {
        // Fits into the current row
        row.push(lItem);
        return [];
    }

    if (row.length > 0 && -deltaWidth < newItemWidth * 0.5) {
        // Overflows by less than half of itself: stretch the row without it, start a new one
        const closed = closeRow(row, vp);
        lItem.row = state.rowNum;
        state.openRow = [lItem];
        return closed;
    }

    // Overflows a lot: add it and shrink the row
    row.push(lItem);
    const closed = closeRow(row, vp);
    state.openRow = [];
    return closed;
}

/** Stretches the row to exactly screenWidth and moves layout to the next row */
function closeRow(row: LayoutItem[], vp: UpdateLayoutPayload): LayoutItem[] {
    const natural = row.reduce((sum, itm) => sum + naturalWidth(itm, vp.rowHeight), 0);
    const height = vp.rowHeight * (vp.screenWidth - gapsForRow(row.length)) / natural;

    let x = 0;
    for (const itm of row) {
        itm.h = height;
        itm.w = itm.width * height / itm.height;
        itm.x = x;
        itm.y = state.y;
        itm.scale = height / itm.height;
        x += itm.w + GUTTER;
    }
    // Rounding: the last item absorbs the remainder so the row is exactly full
    const last = row[row.length - 1];
    last.w = vp.screenWidth - last.x;

    state.y += height + GUTTER;
    state.rowNum++;
    return row;
}

/** The open (last, incomplete) row stays at the target height, left-aligned */
function placeOpenRow(vp: UpdateLayoutPayload): LayoutItem[] {
    let x = 0;
    for (const itm of state.openRow) {
        itm.h = vp.rowHeight;
        itm.w = naturalWidth(itm, vp.rowHeight);
        itm.x = x;
        itm.y = state.y;
        itm.row = state.rowNum;
        itm.scale = vp.rowHeight / itm.height;
        x += itm.w + GUTTER;
    }
    return [...state.openRow];
}
