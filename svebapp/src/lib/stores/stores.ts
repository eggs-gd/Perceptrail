import {writable} from "svelte/store";
import type {Item} from "$lib/gallery";
import {liveQuery} from "dexie";
import {itemsDb} from "./idb";


//let items = dexieStore(async () => await db.items.toArray());
const items = liveQuery(async () => await itemsDb.items.toArray());

const screenWidth = writable<number>(1280);

const rowHeight = writable<number>(220);
const currentIndex = writable<number>(0);
const currentItem = writable<Item>();


export {items, currentIndex, currentItem};
