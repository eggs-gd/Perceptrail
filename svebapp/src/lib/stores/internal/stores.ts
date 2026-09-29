import type {Item} from "./types";
import {writable} from "svelte/store";

// The layout itself is not in a store: the gallery reads the visible window from
// layoutDb with liveQuery (see Gallery.svelte, findings "liveQuery across threads").

export const screenWidth = writable<number>(1280);
export const rowHeight = writable<number>(220);
export const currentIndex = writable<number>(0);
export const currentItem = writable<Item>();

export const currentPage = writable<number>(0);
export const pageSize = writable<number>(20);
