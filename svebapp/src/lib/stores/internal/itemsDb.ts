import Dexie, {type EntityTable} from "dexie";
import type {Item, LayoutOrder} from "./types";

/** Where the delta sync is: the server's epoch and the cursor of the last sync */
export interface SyncState {
    key: 'state';
    epoch: string;
    cursor: string;
}

export const itemsDb: Dexie & {
    items: EntityTable<Item, 'guid'>;
    sync: EntityTable<SyncState, 'key'>;
    orders: EntityTable<LayoutOrder, 'key'>;
} = new Dexie('items') as Dexie & {
    items: EntityTable<Item, 'guid'>;
    sync: EntityTable<SyncState, 'key'>;
    orders: EntityTable<LayoutOrder, 'key'>;
};
// v4: the items are kept between visits; 'sync' holds the delta's cursor
itemsDb.version(4).stores({
    items: '&guid, id, width, height, mimeType, date',
    sync: '&key',
});
// v5: 'orders' — each view's last order from the server, the same for every tab (the
// layout itself is per tab, see layoutDb)
itemsDb.version(5).stores({
    orders: '&key',
});
// Written by wsync only — not on import: several contexts import this module.
