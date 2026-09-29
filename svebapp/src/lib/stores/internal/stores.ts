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

// The worker streams one message per item. Applying each one separately re-sorts
// and re-renders the whole gallery per item and starves the main thread (clicks,
// resize). Queue them and apply everything received so far once per frame.
let pending: MessageFromSync[] = [];
let frameRequested = false;

function onUpdateLayout(message: MessageFromSync) {
    pending.push(message);
    if (!frameRequested) {
        frameRequested = true;
        requestAnimationFrame(flushLayoutUpdates);
    }
}

function flushLayoutUpdates() {
    frameRequested = false;
    let batch = pending;
    pending = [];

    // A replace carries the complete layout: anything queued before it is obsolete
    // (e.g. several resizes within one frame)
    const lastReplace = batch.findLastIndex((m) => m.action === 'replace');
    if (lastReplace > 0) {
        batch = batch.slice(lastReplace);
    }

    layoutItems.update((items) => {
        for (const message of batch) {
            switch (message.action) {
                case 'reset':
                    items = new Map();
                    break;
                case 'replace':
                    items = new Map(message.items!.map((itm) => [itm.guid, itm]));
                    break;
                case 'upsert':
                    for (const itm of message.items!) {
                        items.set(itm.guid, itm);
                    }
                    break;
                case 'create':
                case 'update':
                    items.set(message.item!.guid, message.item as LayoutItem);
                    break;
                case 'delete':
                    items.delete(message.item!.guid);
                    break;
            }
        }
        return items;
    });
}
