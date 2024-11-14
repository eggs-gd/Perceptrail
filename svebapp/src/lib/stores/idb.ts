import Dexie, {type EntityTable, liveQuery} from "dexie";
import type {Item} from "$lib/gallery";
import {type Readable} from 'svelte/store'

interface SvItem extends Item {
    date: Date;
}

let db: Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};

db = new Dexie('myDatabase') as Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};
db.version(2).stores({
    items: '&guid, width, height, mimeType, date, path'
});
db.table("items").clear()

export function dexieStore<T>(querier: () => T | Promise<T>): Readable<T> {
    const dexieObservable = liveQuery(querier)
    return {
        subscribe(run, invalidate) {
            return dexieObservable.subscribe(run, invalidate).unsubscribe

        }
    }
}

export {type SvItem, db}
