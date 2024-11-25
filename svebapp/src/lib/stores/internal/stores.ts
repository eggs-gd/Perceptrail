import type {Item, LayoutItem} from "./types";
import {liveQuery} from "dexie";
import {layoutDb} from "./layoutDb";
import {writable, derived, fromStore} from "svelte/store";
import {QueryRune} from "$lib/stores/internal/queryrune.svelte";

export const screenWidth = writable<number>(1280);
export const rowHeight = writable<number>(220);
export const currentIndex = writable<number>(0);
export const currentItem = writable<Item>();

export const currentPage = writable<number>(0);
export const pageSize = writable<number>(20);

export const items = liveQuery(() => layoutDb.items.orderBy('order').toArray());

export function liveRune<T>(
    querier: () => T | Promise<T>,
    ...dependencies: any[]
): QueryRune<T> | { current: undefined } {
    if (!dependencies.every((x) => x)) {
        return { current: undefined }
    }

    return new QueryRune(liveQuery(querier))
}

// export const paginatedItems = derived(
//     [currentPage, pageSize],
//     ([$currentPage, $pageSize]) =>
//         liveQuery(() =>
//             layoutDb.items
//                 .orderBy("order")
//                 .offset($currentPage * $pageSize)
//                 .limit($pageSize)
//                 .toArray()
//         )
// );
