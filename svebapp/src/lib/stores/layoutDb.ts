import Dexie, {type EntityTable} from "dexie";
import type {Item} from "$lib/gallery";

export interface LayoutItem extends Item {
    order: number;
    scale: number;
    row: number;
}

export const layoutDb: Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
} = new Dexie('layout') as Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
};
layoutDb.version(1).stores({
    items: '&guid, width, height, mimeType, date, order, scale, row'
});
layoutDb.table("items").clear()
