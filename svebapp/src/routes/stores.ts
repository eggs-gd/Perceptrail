import {writable} from "svelte/store";
import type {Item} from "$lib/gallery";

export const items = writable<Item[]>();
export const currentIndex = writable<number>();
export const currentItem = writable<Item>();
