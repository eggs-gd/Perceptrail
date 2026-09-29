import Dexie, {type EntityTable} from "dexie";
import type {Item} from "./types";

export const itemsDb: Dexie & {
    items: EntityTable<Item, 'guid'>;
} = new Dexie('items') as Dexie & {
    items: EntityTable<Item, 'guid'>;
};
itemsDb.version(3).stores({
    items: '&guid, id, width, height, mimeType, date'
});
// Cleared by wsync at the start of a sync — not on import: several contexts import
// this module, and a clear here would race with wsync's writes.
