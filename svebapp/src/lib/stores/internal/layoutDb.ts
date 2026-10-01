import Dexie, {type EntityTable} from "dexie";
import type {LayoutItem, LayoutMeta, LayoutSections, LayoutSize} from "./types";

export const LAYOUT_META_KEY = 'layout' as const;
export const LAYOUT_SIZE_KEY = 'size' as const;
export const LAYOUT_SECTIONS_KEY = 'sections' as const;

// Written by the layout worker only; the page reads it with liveQuery over the
// visible window (see Gallery.svelte). No clear() on import: several contexts
// import this module, and a clear here would race with the worker's writes.
export const layoutDb: Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
    meta: EntityTable<LayoutMeta | LayoutSize | LayoutSections, 'key'>;
} = new Dexie('layout') as Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
    meta: EntityTable<LayoutMeta | LayoutSize | LayoutSections, 'key'>;
};
// v3: positions indexed by y and bottom (visible-window query) and order (viewer);
// meta holds three records: 'layout' (per relayout), 'size' (per streamed batch) and
// 'sections' (the side panel's marks, with either)
layoutDb.version(3).stores({
    items: '&guid, order, y, bottom',
    meta: '&key',
});
