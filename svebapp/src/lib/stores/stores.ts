import {writable} from "svelte/store";
import type {Item} from "$lib/gallery";
import {liveQuery} from "dexie";
import {db} from "./idb";


//let items = dexieStore(async () => await db.items.toArray());
const items = liveQuery(async () => await db.items.toArray());

const currentIndex = writable<number>();
const currentItem = writable<Item>();


export {items, currentIndex, currentItem};
