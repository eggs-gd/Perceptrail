import {writable} from "svelte/store";
import type {Item} from "$lib/gallery";
import {liveQuery} from "dexie";
import {layoutDb} from "$lib/stores/layoutDb";

export const items = liveQuery(() => layoutDb.items.orderBy('order').toArray());

export const screenWidth = writable<number>(1280);
export const rowHeight = writable<number>(220);
export const currentIndex = writable<number>(0);
export const currentItem = writable<Item>();
