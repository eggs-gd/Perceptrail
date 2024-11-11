import Dexie, {type EntityTable, liveQuery} from "dexie";
import type {Item} from "$lib/gallery";
import {type Readable} from 'svelte/store'
import {browser} from "$app/environment";
import {indexedDB as fakeIdb, IDBKeyRange as fakeIdbK} from "fake-indexeddb";

interface SvItem extends Item {
    date: Date;
}

let db: Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};

if (browser) {
    db = new Dexie('myDatabase') as Dexie & {
        items: EntityTable<SvItem, 'guid'>;
    };
    db.version(2).stores({
        items: '&guid, width, height, mimeType, date, path'
    });
    db.table("items").clear()
} else {
    db = new Dexie('myDatabase', {indexedDB: fakeIdb, IDBKeyRange: fakeIdbK}) as Dexie & {
        items: EntityTable<SvItem, 'guid'>;
    };
    db.version(2).stores({
        items: '&guid, width, height, mimeType, date, path'
    });
}


export function dexieStore<T>(querier: () => T | Promise<T>): Readable<T> {
    const dexieObservable = liveQuery(querier)
    return {
        subscribe(run, invalidate) {
            return dexieObservable.subscribe(run, invalidate).unsubscribe

        }
    }
}

export {type SvItem, db}
