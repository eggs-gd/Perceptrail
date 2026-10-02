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
// sections). The page names its database (`layout-<random>`); its layout worker gets
// the name with 'init'. Databases of closed tabs are dropped by the next page: a live
// page holds a Web Lock of its database's name, or — outside a secure context (plain
// HTTP on a LAN address: no Web Locks) — answers a roll call on a BroadcastChannel.
const PREFIX = 'layout-';
const ROLL_CALL = 'layout-tabs';
const ROLL_CALL_MS = 1000;

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

// Not crypto.randomUUID: secure contexts only
const randomId = () => Date.now().toString(36) + Math.random().toString(36).slice(2);

/** This page's layout database (the worker's after useLayoutDb) */
export let layoutDbName = PREFIX + (inPage ? randomId() : 'unset');

// Written by the layout worker only; the page reads it with liveQuery over the
// visible window (see Gallery.svelte). Not opened until used (SSR never does).
export let layoutDb: LayoutDb = open(layoutDbName);

/** The layout worker: the database its page named */
export function useLayoutDb(name: string) {
    layoutDb.close();
    layoutDbName = name;
    layoutDb = open(name);
}

if (inPage) {
    if (navigator.locks) {
        // Held until the page goes away
        navigator.locks.request(layoutDbName, () => new Promise<never>(() => {}));
    } else {
        const roll = new BroadcastChannel(ROLL_CALL);
        roll.onmessage = (e) => { if (e.data === 'who') roll.postMessage(layoutDbName); };
    }
    // Dropped all the same (a page that missed the roll call — frozen, busy): start
    // again rather than show nothing
    layoutDb.on('versionchange', (e) => {
        if (e.newVersion === null) location.reload();
    });
    dropClosedTabs().catch(() => {});
}

async function dropClosedTabs() {
    if (!indexedDB.databases) return;
    const [dbs, alive] = await Promise.all([indexedDB.databases(), aliveTabs()]);
    for (const {name} of dbs) {
        // 'layout': the shared database before one per tab
        if (name === 'layout' || (name?.startsWith(PREFIX) && name !== layoutDbName && !alive.has(name))) {
            indexedDB.deleteDatabase(name);
        }
    }
}

/** The layout databases of the pages alive now */
async function aliveTabs(): Promise<Set<string>> {
    if (navigator.locks) {
        const locks = await navigator.locks.query();
        return new Set([...(locks.held ?? []), ...(locks.pending ?? [])].map((l) => l.name ?? ''));
    }
    const alive = new Set<string>();
    const roll = new BroadcastChannel(ROLL_CALL);
    roll.onmessage = (e) => alive.add(e.data);
    roll.postMessage('who');
    await new Promise((r) => setTimeout(r, ROLL_CALL_MS));
    roll.close();
    return alive;
}
