import {
    type MessageFromSync,
    type UpdateLayoutPayload,
    type WorkerMessage,
} from "./types";
import {type Item, type LayoutItem} from "$lib/stores";
import {getLogger} from "$lib/logger";

const logger = getLogger()

// Must match Gallery.svelte gutter (gap between items and rows)
const GUTTER = 8;
// Window-edge drag sends a resize per frame; lay out at most this often (~30 fps)
const RELAYOUT_THROTTLE_MS = 33;

let updatesPort: MessagePort;
let viewport: UpdateLayoutPayload | null = null;
let relayoutTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Every item streamed so far, in stream (id) order. Kept in memory so a resize
 * is one synchronous pass: nothing to await, so nothing to abort and no races.
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

function freshState(): LayoutState {
    return {openRow: [], rowNum: 0, y: 0};
}

self.onmessage = function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    if (task === 'init') {
        const itemsDbPort: MessagePort = payload[0];
        itemsDbPort.onmessage = onItemsDbMessage;
        updatesPort = payload[1];
        logger.debug('Inited')
    } else if (task === 'update') {
        const p: UpdateLayoutPayload = payload;
        if (viewport?.screenWidth === p.screenWidth && viewport?.rowHeight === p.rowHeight) {
            return;
        }
        viewport = {screenWidth: p.screenWidth, rowHeight: p.rowHeight};
        scheduleRelayout();
    }
};

function onItemsDbMessage(event: MessageEvent<MessageFromSync>) {
    const {action, item} = event.data;

    switch (action) {
        case 'sync-start':
            items = [];
            relayout();
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
        relayout();
    }, RELAYOUT_THROTTLE_MS);
}

/** Lays out every item from scratch and replaces the whole view in one message */
function relayout() {
    state = freshState();
    if (!viewport) return;

    const all: LayoutItem[] = [];
    for (let i = 0; i < items.length; i++) {
        all.push(...place(items[i], i, viewport));
    }
    all.push(...placeOpenRow(viewport));

    logger.debug('relayout', {items: all.length, width: viewport.screenWidth});
    const msg: MessageFromSync = {action: 'replace', items: all};
    updatesPort.postMessage(msg);
}

/** Places one streamed item and sends only what changed */
function append(item: Item) {
    // No viewport yet or a full relayout is pending: that run will include this item
    if (!viewport || relayoutTimer) return;

    const changed = place(item, items.length - 1, viewport);
    changed.push(...placeOpenRow(viewport));

    const msg: MessageFromSync = {action: 'upsert', items: changed};
    updatesPort.postMessage(msg);
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
