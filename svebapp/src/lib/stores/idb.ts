import Dexie, {type EntityTable, liveQuery} from "dexie";
import type {Item} from "$lib/gallery";
import {type Readable} from 'svelte/store'

interface SvItem extends Item {
    date: Date;
}

interface LayoutItem extends Item {
    order: number;
    scale: number;
    row: number;
}

let itemsDb: Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};

itemsDb = new Dexie('items') as Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};
itemsDb.version(2).stores({
    items: '&guid, width, height, mimeType, date'
});
itemsDb.table("items").clear()

let layoutDb: Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
};

layoutDb = new Dexie('layout') as Dexie & {
    items: EntityTable<LayoutItem, 'guid'>;
};
layoutDb.version(1).stores({
    items: '&guid, width, height, mimeType, date, order, scale, row'
});
layoutDb.table("items").clear()

export function dexieStore<T>(querier: () => T | Promise<T>): Readable<T> {
    const dexieObservable = liveQuery(querier)
    return {
        subscribe(run, invalidate) {
            return dexieObservable.subscribe(run, invalidate).unsubscribe

        }
    }
}

export {type SvItem, itemsDb, type LayoutItem, layoutDb}
