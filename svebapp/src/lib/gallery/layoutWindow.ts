import {liveQuery} from "dexie";
import {
    LAYOUT_META_KEY,
    LAYOUT_SIZE_KEY,
    layoutDb,
    type LayoutItem,
    type LayoutMeta,
    type LayoutSize,
} from "$lib/stores";

/** Items above the window whose bottom may still reach into it (taller than any row) */
const ROW_OVERLAP = 1000;

/**
 * The item kept in view across relayouts. `ratio` = (viewport top − item top) / item
 * height: relative, so several relayouts in a row do not accumulate pixel error.
 */
export interface Anchor {
    guid: string;
    ratio: number;
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
    return liveQuery(() => layoutDb.transaction('r', layoutDb.items, layoutDb.meta, async (): Promise<WindowSnapshot> => {
        const meta = await layoutDb.meta.get(LAYOUT_META_KEY) as LayoutMeta | undefined;

        let {top, bottom} = range;
        let scrollTo: number | undefined;
        const a = anchor.pending;
        if (meta?.anchor && a && meta.rev !== anchor.appliedRev && meta.anchor.guid === a.guid) {
            scrollTo = Math.max(0, meta.anchor.y + a.ratio * meta.anchor.h);
            top = scrollTo - range.viewport;
            bottom = scrollTo + 2 * range.viewport;
        }

        const items = await layoutDb.items
            .where('y').between(top - ROW_OVERLAP, bottom, true, true)
            .sortBy('order');
        return {meta, items, scrollTo};
    }));
}

/** Gallery height/count; changes with every streamed batch, cheap to re-read */
export function watchSize() {
    return liveQuery(() => layoutDb.meta.get(LAYOUT_SIZE_KEY) as Promise<LayoutSize | undefined>);
}

/** First item whose bottom is below the viewport top (items sorted by order) */
export function findAnchor(items: LayoutItem[], viewportTop: number): Anchor | undefined {
    const itm = items.find((i) => i.y + i.h > viewportTop);
    return itm && {guid: itm.guid, ratio: Math.max(0, viewportTop - itm.y) / itm.h};
}
