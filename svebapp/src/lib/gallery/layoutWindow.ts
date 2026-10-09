import {liveQuery} from "dexie";
import {
    LAYOUT_META_KEY,
    LAYOUT_SIZE_KEY,
    layoutDb,
    type LayoutItem,
    type LayoutMeta,
    type LayoutSize,
} from "$lib/stores";

/**
 * What stays pinned on screen across relayouts (resize):
 * - top:    at the very top of the page — the first photo stays at the top;
 * - bottom: at the very bottom — the end of the gallery stays at the bottom;
 * - center: anywhere else — the photo in the middle of the screen stays there.
 */
export type AnchorMode = 'top' | 'center' | 'bottom';

/** Where on the screen the anchor is pinned: fraction of the viewport height */
const PIN_POINT: Record<AnchorMode, number> = {top: 0, center: 0.5, bottom: 1};

export interface Anchor {
    mode: AnchorMode;
    /** Pinned item (top/center); bottom pins the end of the gallery instead */
    guid?: string;
    /**
     * (pin point − item top) / item height: relative, so several relayouts in a row
     * do not accumulate pixel error
     */
    ratio?: number;
}

/** Shared between the page (writes) and the querier (reads at run time) */
export interface AnchorState {
    /**
     * Taken at the first resize and kept across all following ones until the user
     * scrolls: re-taking it after every resize walks the view (after a reflow the
     * top-left photo is often an earlier one than the anchor).
     */
    pending?: Anchor;
    /** meta.rev whose anchor has already been scrolled to */
    appliedRev: number;
}

export interface WindowRange {
    top: number;
    bottom: number;
    viewport: number;
}

export interface WindowSnapshot {
    meta?: LayoutMeta;
    items: LayoutItem[];
    /** Set once per relayout of the pending anchor: scroll here (gallery coordinates) */
    scrollTo?: number;
    /** Bottom anchor: scroll to the real end of the document (includes page padding) */
    toEnd?: boolean;
}

export async function readWindow(range: WindowRange, anchor: AnchorState): Promise<WindowSnapshot> {
    return layoutDb.transaction('r', layoutDb.items, layoutDb.meta, async (): Promise<WindowSnapshot> => {
        const meta = await layoutDb.meta.get(LAYOUT_META_KEY) as LayoutMeta | undefined;

        let {top, bottom} = range;
        let scrollTo: number | undefined;
        const a = anchor.pending;
        // top: the page stays at 0 — no correction needed at all
        if (meta && a && a.mode !== 'top' && meta.rev !== anchor.appliedRev) {
            if (a.mode === 'bottom') {
                scrollTo = Math.max(0, meta.height - range.viewport);
            } else if (meta.anchor && meta.anchor.guid === a.guid) {
                const pinned = meta.anchor.y + a.ratio! * meta.anchor.h;
                scrollTo = Math.max(0, pinned - PIN_POINT[a.mode] * range.viewport);
            }
        }
        if (scrollTo !== undefined) {
            top = scrollTo - range.viewport;
            bottom = scrollTo + 2 * range.viewport;
        }

        const items = await itemsInRange(top, bottom);
        return {meta, items, scrollTo, toEnd: a?.mode === 'bottom' && scrollTo !== undefined};
    });
}

/**
 * One liveQuery over the visible window. The 'layout' meta record and the items are
 * read in ONE read transaction, so every emission is a consistent snapshot. When
 * meta carries a new relayout of the pending anchor, the window is taken around the
 * anchor's new position and returned with scrollTo — items and scroll change in the
 * same update. It does not read 'size' (changes per streamed batch), and writes
 * outside the window do not re-run it.
 */
export function watchWindow(range: WindowRange, anchor: AnchorState) {
    return liveQuery(() => readWindow(range, anchor));
}

/**
 * Rows intersecting [top, bottom). Rows are stacked, so y and bottom grow with order:
 * find the first row that reaches into the window, then take every row that starts
 * before its end. No fixed overlap — a single row can be taller than the window
 * (a portrait closed alone is stretched to the full width). Every range read is
 * bounded, so streamed photos below the window do not re-run the query.
 */
async function itemsInRange(top: number, bottom: number): Promise<LayoutItem[]> {
    let first = await layoutDb.items.where('bottom').between(top, bottom, false, true).first();
    if (!first) {
        // No row ends inside the window: maybe one row covers all of it
        const above = await layoutDb.items.where('y').belowOrEqual(top).last();
        if (above && above.bottom > top) first = above;
    }
    if (!first) return [];
    return layoutDb.items.where('y').between(first.y, bottom, true, false).sortBy('order');
}

/** Gallery height/count; changes with every streamed batch, cheap to re-read */
export function watchSize() {
    return liveQuery(() => layoutDb.meta.get(LAYOUT_SIZE_KEY) as Promise<LayoutSize | undefined>);
}

export interface View {
    /** Viewport top in gallery coordinates */
    top: number;
    height: number;
    width: number;
}

/**
 * The anchor for a relayout: its mode depends on where the page is scrolled (the
 * edges pin the edge, the middle pins the photo in the middle of the screen).
 */
export function findAnchor(items: LayoutItem[], view: View, edge: 'top' | 'bottom' | null): Anchor | undefined {
    // Edges pin the edge of the page itself, no item needed
    if (edge) return {mode: edge};
    const mode: AnchorMode = 'center';
    const point = view.top + PIN_POINT[mode] * view.height;

    // The row at the pin point (or the next one, if the point falls into a gap)
    const below = items.filter((i) => i.bottom > point);
    if (below.length === 0) return undefined;
    const rowY = Math.min(...below.map((i) => i.y));
    const row = below.filter((i) => i.y === rowY);

    // The photo under the middle of the screen
    const x = view.width / 2;
    const distance = (i: LayoutItem) => x < i.x ? i.x - x : x > i.x + i.w ? x - (i.x + i.w) : 0;
    const itm = row.reduce((best, i) => distance(i) < distance(best) ? i : best);
    return {mode, guid: itm.guid, ratio: (point - itm.y) / itm.h};
}
