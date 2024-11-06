import "fake-indexeddb/auto";

import Dexie, {type EntityTable} from 'dexie';
import {writable} from "svelte/store";

import type {SvItem} from "$lib/server/server.api";
import type {Item} from "$lib/gallery";


const currentIndex = writable<number>();
const currentItem = writable<Item>();


const db = new Dexie('myDatabase') as Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};
db.version(1).stores({
    items: '&guid, width, height, mimeType, date'
});

export {db, currentIndex, currentItem};
