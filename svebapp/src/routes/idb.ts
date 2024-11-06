import Dexie, {type EntityTable} from "dexie";
import type {Item} from "$lib/gallery";
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
} else {
    db = new Dexie('myDatabase', {indexedDB: fakeIdb, IDBKeyRange: fakeIdbK}) as Dexie & {
        items: EntityTable<SvItem, 'guid'>;
    };
    db.version(2).stores({
        items: '&guid, width, height, mimeType, date, path'
    });
}

export {type SvItem, db}
