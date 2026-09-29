import {liveQuery} from "dexie";
import {LAYOUT_META_KEY, layoutDb, type LayoutItem, type LayoutMeta} from "$lib/stores";

/** Items above the window whose bottom may still reach into it (taller than any row) */
const ROW_OVERLAP = 1000;

/** The first visible item before a relayout; offset = viewport top − item top, px */
export interface Anchor {
    guid: string;
    offset: number;
    h: number;
}

/** Shared between the page (writes) and the querier (reads at run time) */
export interface AnchorState {
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
}

/**
 * One liveQuery over the visible window. meta and items are read in ONE read
 * transaction, so every emission is a consistent snapshot. When meta carries a new
 * relayout of the pending anchor, the window is taken around the anchor's new
 * position and returned with scrollTo — items and scroll change in the same update.
 * Writes outside the window do not re-run it (spike: 0 re-runs once filled).
 */
export function watchWindow(range: WindowRange, anchor: AnchorState) {
    return liveQuery(() => layoutDb.transaction('r', layoutDb.items, layoutDb.meta, async (): Promise<WindowSnapshot> => {
        const meta = await layoutDb.meta.get(LAYOUT_META_KEY);

        let {top, bottom} = range;
        let scrollTo: number | undefined;
        const a = anchor.pending;
        if (meta?.anchor && a && meta.rev !== anchor.appliedRev && meta.anchor.guid === a.guid) {
            scrollTo = Math.max(0, meta.anchor.y + a.offset * (meta.anchor.h / a.h));
            top = scrollTo - range.viewport;
            bottom = scrollTo + 2 * range.viewport;
        }

        const items = await layoutDb.items
            .where('y').between(top - ROW_OVERLAP, bottom, true, true)
            .sortBy('order');
        return {meta, items, scrollTo};
    }));
}

/** First item whose bottom is below the viewport top (items sorted by order) */
export function findAnchor(items: LayoutItem[], viewportTop: number): Anchor | undefined {
    const itm = items.find((i) => i.y + i.h > viewportTop);
    return itm && {guid: itm.guid, offset: Math.max(0, viewportTop - itm.y), h: itm.h};
}
