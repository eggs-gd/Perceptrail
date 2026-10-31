import Dexie from "dexie";
import {type Item, itemsDb} from "#lib/stores/index.js";
import {type CurrentWorkerTask, ITEMS_CHANNEL, type MessageFromSync, type WorkerMessage} from "./types";
import {getLogger, setLogLevel} from "#lib/logger/index.js";
import {open, store} from "./stream";

const logger = getLogger();
let currentTask: CurrentWorkerTask = null;

// To every tab's layout worker (see ITEMS_CHANNEL)
const updatesPort = new BroadcastChannel(ITEMS_CHANNEL);

// A dedicated worker: only its page posts here (and a worker's message has no origin)
globalThis.onmessage = async function (msg: { data: WorkerMessage<any, any> }) { // NOSONAR
    const {task, payload} = msg.data;

    if (task === 'mode') {
        setLogLevel(payload);
    } else if (task === 'init') {
        logger.debug('Inited');
    } else if (task === 'start' && !currentTask) {
        currentTask = startNewTask(payload);
        logger.debug('Started')
    } else if (task === 'start' && currentTask) {
        again = true; // folded into one more run after this one
    }
};

/**
 * One sync. The items are kept between visits: with a cursor from the same server
 * database (its epoch) only what changed since comes (/items?since=), a removed item
 * as {guid, removed}. Otherwise everything comes — and how it lands depends on the
 * epoch, "<database>.<contract>":
 * - another database (or nothing kept): from an empty table — the copy is not ours;
 * - the same database, another contract (what an item carries changed): over the
 *   kept copy, in place — every item put, the ones the stream did not bring deleted
 *   after it. The sheet does not empty, a direct link keeps working.
 * Starts asked for while one runs are folded into one more run after it.
 *
 * Every tab has its own sync worker over the one IndexedDB: a full sync in one tab
 * cleared the table while another was filling it, and that one kept its cursor over
 * what was left (a library of 79 photos instead of 6 993). Syncs take a lock shared by
 * the tabs (Web Locks), and the stream's last line says how many items there are: a
 * copy holding another count is synced again in place.
 */
function startNewTask(apiPath: string) {
    const controller = new AbortController();
    return {
        controller,
        promise: (async () => {
            try {
                // No Web Locks outside a secure context (plain HTTP on a LAN address):
                // unserialized then, and the count check heals what two tabs break
                const changed = navigator.locks
                    ? await navigator.locks.request('items-sync', {signal: controller.signal},
                        () => sync(controller.signal, apiPath))
                    : await sync(controller.signal, apiPath);
                postMessage({task: 'sync', status: 'completed', changed});
            } catch (error) {
                postMessage({task: 'sync', status: 'error', changed: 0});
                logger.error("Error in UpdateDb task:", error);
            } finally {
                currentTask = null;
                if (again) {
                    again = false;
                    currentTask = startNewTask(apiPath);
                }
            }
        })(),
    };
}

let again = false;

/** The database part of an epoch ("<database>.<contract>") */
const databaseOf = (epoch: string) => epoch.split('.')[0];

/** refetch: everything, in place (the copy did not match the server's count) */
async function sync(signal: AbortSignal, apiPath: string, refetch = false): Promise<number> {
    const state = await itemsDb.sync.get('state');
    const {response, epoch, delta} = await open(signal, apiPath, refetch ? undefined : state);
    if (!response.ok) throw new Error(`${apiPath}: ${response.status}`);

    // Everything, from an empty table: only when the copy is not this database's
    const replace = !delta && (!state || databaseOf(state.epoch) !== databaseOf(epoch));
    // Everything over the kept copy: what the stream does not bring is gone
    const seen = !delta && !replace ? new Set<string>() : undefined;
    if (replace) {
        // Before the hooks, or clear() would report every leftover row as a delete
        await itemsDb.items.clear();
        updatesPort.postMessage({action: 'sync-start'});
    }
    itemsDb.items.hook.creating.subscribe(hookCreate);
    itemsDb.items.hook.updating.subscribe(hookUpdate);
    itemsDb.items.hook.deleting.subscribe(hookDelete);
    let stored: {changed: number, cursor: string, total: number};
    try {
        stored = await store(signal, response, seen);
    } finally {
        itemsDb.items.hook.creating.unsubscribe(hookCreate);
        itemsDb.items.hook.updating.unsubscribe(hookUpdate);
        itemsDb.items.hook.deleting.unsubscribe(hookDelete);
    }
    const {cursor, total} = stored;
    let {changed} = stored;
    // The cursor is the stream's last line: none means it was cut short (the server
    // failed after the 200, the connection dropped) — then the old cursor stays: a
    // failed sync starts again from it
    if (!cursor) throw new Error(`${apiPath}: the stream ended without its cursor`);
    if (replace) updatesPort.postMessage({action: 'sync-done'});
    const held = await itemsDb.items.count();
    if (total >= 0 && held !== total) {
        if (delta) {
            // The copy is not what the server has: the delta cannot mend it
            logger.warn(`Sync: ${held} items kept, the server has ${total} — syncing everything in place`);
            return changed + await sync(signal, apiPath, true);
        }
        // A full sync can differ only by what changed while it ran: the next delta brings it
        logger.warn(`Sync: ${held} items after a full sync, the server had ${total}`);
    }
    if (epoch) await itemsDb.sync.put({key: 'state', epoch, cursor});
    return changed;
}

function hookCreate(key: string, item: Item) {
    const msg: MessageFromSync = {action: "create", item: item};
    updatesPort.postMessage(msg);
}

function hookUpdate(mods: Record<string, unknown>, key: string, item: Item) {
    // Dexie passes the pre-update object and the changes by key path
    // ("asset.original": …) — applied by path: spread over the object they would sit
    // beside it as keys with dots, and a change inside the asset (an original that
    // became local) never reached the layout until a reload
    if (Object.keys(mods).length === 0) return;
    const updated = structuredClone(item);
    for (const [path, value] of Object.entries(mods)) {
        if (value === undefined) Dexie.delByKeyPath(updated, path);
        else Dexie.setByKeyPath(updated, path, value);
    }
    const msg: MessageFromSync = {action: "update", item: updated};
    updatesPort.postMessage(msg);
}

function hookDelete(key: string, item: Item) {
    const msg: MessageFromSync = {action: "delete", item: item};
    updatesPort.postMessage(msg);
}
