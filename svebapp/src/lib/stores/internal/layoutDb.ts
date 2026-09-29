import Dexie, {type EntityTable} from "dexie";
import type {LayoutItem, LayoutMeta} from "./types";

export const LAYOUT_META_KEY = 'layout' as const;

// Written by the layout worker only; the page reads it with liveQuery over the
// visible window (see Gallery.svelte). No clear() on import: several contexts
// import this module, and a clear here would race with the worker's writes.
export const layoutDb: Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
    meta: EntityTable<LayoutMeta, 'key'>;
} = new Dexie('layout') as Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
    meta: EntityTable<LayoutMeta, 'key'>;
};
// v2: positions indexed by y (visible-window query) and order (viewer), meta record
layoutDb.version(2).stores({
    items: '&guid, order, y',
    meta: '&key',
});
