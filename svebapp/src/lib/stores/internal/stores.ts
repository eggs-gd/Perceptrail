import type {Item, LayoutItem} from "./types";
import {liveQuery} from "dexie";
import {layoutDb} from "./layoutDb";
import {derived, writable} from "svelte/store";
import type {MessageFromSync} from "$lib/workers/tasks/types";

let layoutUpdatesPort: MessagePort;

export const screenWidth = writable<number>(1280);
export const rowHeight = writable<number>(220);
export const currentIndex = writable<number>(0);
export const currentItem = writable<Item>();

export const currentPage = writable<number>(0);
export const pageSize = writable<number>(20);


// export const items = liveQuery(async () =>
//     await layoutDb.items.orderBy('order').toArray()
// );

export const layoutItems = writable<Map<string, LayoutItem>>(new Map());


export const items = derived(layoutItems, ($layoutItems) => {
    return Array.from($layoutItems.values()).sort((a, b) => a.order - b.order);
});

export const updateLayoutPort = (port: MessagePort) => {
    layoutUpdatesPort = port;
    layoutUpdatesPort.onmessage = (event: MessageEvent<MessageFromSync>) => onUpdateLayout(event.data);
}

function onUpdateLayout(message: MessageFromSync) {
    layoutItems.update((items) => {
        switch (message.action) {
            case 'create':
            case 'update':
                items.set(message.item.guid, message.item as LayoutItem);
                break;
            case 'delete':
                items.delete(message.item.guid);
                break;
        }
        return items;
    });
}
