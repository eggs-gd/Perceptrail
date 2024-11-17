import Dexie, {type EntityTable} from "dexie";
import type {Item} from "$lib/gallery";

export interface SvItem extends Item {
    date: Date;
}

export const itemsDb: Dexie & {
    items: EntityTable<SvItem, 'guid'>;
} = new Dexie('items') as Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};
itemsDb.version(2).stores({
    items: '&guid, width, height, mimeType, date'
});
itemsDb.table("items").clear()
