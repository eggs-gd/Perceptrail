import Dexie, {type EntityTable} from "dexie";
import type {Item} from "./types";

export const itemsDb: Dexie & {
    items: EntityTable<Item, 'guid'>;
} = new Dexie('items') as Dexie & {
    items: EntityTable<Item, 'guid'>;
};
itemsDb.version(2).stores({
    items: '&guid, width, height, mimeType, date'
});
itemsDb.table("items").clear()
