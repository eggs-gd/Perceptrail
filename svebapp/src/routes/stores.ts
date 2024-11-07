import {writable} from "svelte/store";
import type {Item} from "$lib/gallery";


const currentIndex = writable<number>();
const currentItem = writable<Item>();


export {currentIndex, currentItem};
