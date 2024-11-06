import {writable} from "svelte/store";
import Dexie, {type EntityTable} from 'dexie';
import type {SvItem} from "$lib/server/server.api";
import type {Item} from "$lib/gallery";

const currentIndex = writable<number>();
const currentItem = writable<Item>();


const db = new Dexie('myDatabase') as Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};
db.version(1).stores({
    items: '&guid, width, height'
});

export async function initializeStore(initialData: SvItem[]) {
    try {
        await db.items.bulkPut(initialData);
    } catch (error) {
        //todo
    }

}

export async function updateFromServer(newData: SvItem[]) {
    try {
        await db.items.bulkPut(newData);
    } catch (error) {
        //todo
    }
}

export {db, currentIndex, currentItem};
