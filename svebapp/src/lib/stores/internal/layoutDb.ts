import Dexie, {type EntityTable} from "dexie";
import type {LayoutItem, LayoutMeta, LayoutSections, LayoutSize} from "./types";

export const LAYOUT_META_KEY = 'layout' as const;
export const LAYOUT_SIZE_KEY = 'size' as const;
export const LAYOUT_SECTIONS_KEY = 'sections' as const;

type LayoutDb = Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
    meta: EntityTable<LayoutMeta | LayoutSize | LayoutSections, 'key'>;
};

// One layout per tab: the positions depend on the tab's width and view. A shared
// database let one tab show another's sheet (a date tab with the place view's
// sections). The page names its database (`layout-<random>`) and holds a Web Lock of
// that name while it lives; its layout worker gets the name with 'init'. Databases
// whose lock nobody holds belong to closed tabs: the next page drops them.
const PREFIX = 'layout-';

function open(name: string): LayoutDb {
    const db = new Dexie(name) as LayoutDb;
    // positions indexed by y and bottom (visible-window query) and order (viewer);
    // meta: 'layout' (per relayout), 'size' (per streamed batch), 'sections' (the side
    // panel's marks)
    db.version(1).stores({
        items: '&guid, order, y, bottom',
        meta: '&key',
    });
    return db;
}

const inPage = typeof window !== 'undefined';

/** This page's layout database (the worker's after useLayoutDb) */
export let layoutDbName = inPage ? PREFIX + crypto.randomUUID() : PREFIX + 'unset';

// Written by the layout worker only; the page reads it with liveQuery over the
// visible window (see Gallery.svelte). Not opened until used (SSR never does).
export let layoutDb: LayoutDb = open(layoutDbName);

/** The layout worker: the database its page named */
export function useLayoutDb(name: string) {
    layoutDb.close();
    layoutDbName = name;
    layoutDb = open(name);
}

if (inPage && navigator.locks) {
    // Held until the page goes away
    navigator.locks.request(layoutDbName, () => new Promise<never>(() => {}));
    dropClosedTabs().catch(() => {});
}

async function dropClosedTabs() {
    if (!indexedDB.databases) return;
    const [dbs, locks] = await Promise.all([indexedDB.databases(), navigator.locks.query()]);
    const alive = new Set([...(locks.held ?? []), ...(locks.pending ?? [])].map((l) => l.name));
    for (const {name} of dbs) {
        // 'layout': the shared database before one per tab
        if (name === 'layout' || (name?.startsWith(PREFIX) && name !== layoutDbName && !alive.has(name))) {
            indexedDB.deleteDatabase(name);
        }
    }
}
