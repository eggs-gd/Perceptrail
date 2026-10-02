import {type Item, itemsDb} from "$lib/stores";
import {type CurrentWorkerTask, type MessageFromSync, type WorkerMessage} from "./types";
import {getLogger, setLogLevel} from "$lib/logger";

const logger = getLogger();
let currentTask: CurrentWorkerTask = null;

let updatesPort: MessagePort;

self.onmessage = async function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    if (task === 'mode') {
        setLogLevel(payload);
    } else if (task === 'init') {
        updatesPort = payload[0];
        logger.debug('Inited');
    } else if (task === 'start' && !currentTask) {
        currentTask = startNewTask(payload);
        logger.debug('Started')
    } else if (task === 'start' && currentTask) {
        again = true; // folded into one more run after this one
    } else if (task === 'update' && currentTask) {
        //todo add to queue
        //todo add logic to manage queue to keep only one last request
    }
};

/**
 * One sync. The items are kept between visits: with a cursor from the same server
 * database (its epoch) only what changed since comes (/items?since=), a removed item
 * as {guid, removed}; otherwise everything, from an empty table. Starts asked for
 * while one runs are folded into one more run after it.
 */
function startNewTask(apiPath: string) {
    const controller = new AbortController();
    return {
        controller,
        promise: (async () => {
            try {
                const changed = await sync(controller.signal, apiPath);
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

async function sync(signal: AbortSignal, apiPath: string): Promise<number> {
    const state = await itemsDb.sync.get('state');
    let response = await fetch(state ? `${apiPath}?since=${encodeURIComponent(state.cursor)}` : apiPath, {signal});
    let epoch = response.headers.get('X-Sync-Epoch') ?? '';
    let full = !state;
    if (state && epoch !== state.epoch) {
        // Another database: our copy is not a base for its delta
        await response.body?.cancel();
        response = await fetch(apiPath, {signal});
        epoch = response.headers.get('X-Sync-Epoch') ?? '';
        full = true;
    }
    if (!response.ok) throw new Error(`${apiPath}: ${response.status}`);

    if (full) {
        // From an empty table. Before the hooks, or clear() would report every
        // leftover row as a delete.
        await itemsDb.items.clear();
        updatesPort.postMessage({action: 'sync-start'});
    }
    itemsDb.items.hook.creating.subscribe(hookCreate);
    itemsDb.items.hook.updating.subscribe(hookUpdate);
    itemsDb.items.hook.deleting.subscribe(hookDelete);
    let changed = 0;
    let cursor = '';
    try {
        await readLines(signal, response, async (line) => {
            if ('cursor' in line) {
                cursor = line.cursor;
                return;
            }
            if (line.removed) await itemsDb.items.delete(line.guid);
            else await itemsDb.items.put(line as Item);
            changed++;
        });
    } finally {
        itemsDb.items.hook.creating.unsubscribe(hookCreate);
        itemsDb.items.hook.updating.unsubscribe(hookUpdate);
        itemsDb.items.hook.deleting.unsubscribe(hookDelete);
    }
    // The cursor is the stream's last line: none means it was cut short (the server
    // failed after the 200, the connection dropped). Kept only once everything is in:
    // a failed sync starts again from the old cursor.
    if (!cursor) throw new Error(`${apiPath}: the stream ended without its cursor`);
    if (epoch) await itemsDb.sync.put({key: 'state', epoch, cursor});
    if (full) updatesPort.postMessage({action: 'sync-done'});
    return changed;
}

function hookCreate(key: string, item: Item) {
    const msg: MessageFromSync = {action: "create", item: item};
    updatesPort.postMessage(msg);
}

function hookUpdate(mods: Object, key: string, item: Item) {
    // Dexie passes the pre-update object: merge mods so the change gets through
    if (Object.keys(mods).length === 0) return;
    const msg: MessageFromSync = {action: "update", item: {...item, ...mods}};
    updatesPort.postMessage(msg);
}

function hookDelete(key: string, item: Item) {
    const msg: MessageFromSync = {action: "delete", item: item};
    updatesPort.postMessage(msg);
}

type SyncLine = (Item & {removed?: boolean}) | {cursor: string};

/**
 * NDJSON, line by line as it streams. A line that cannot be read or stored fails the
 * whole read: the sync keeps its old cursor and retries.
 */
async function readLines(signal: AbortSignal, response: Response,
                         each: (line: SyncLine) => Promise<void>): Promise<void> {
    if (!response.body) throw new Error('Streaming data is not supported');
    const reader = response.body.getReader();
    const decoder = new TextDecoder("utf-8");
    let buffer = "";

    const take = async (text: string) => {
        if (!text.trim()) return;
        await each(JSON.parse(text));
    };

    while (true) {
        signal.throwIfAborted();
        const {done, value} = await reader.read();
        buffer += decoder.decode(value, {stream: !done});
        let boundary;
        while ((boundary = buffer.indexOf("\n")) !== -1) {
            await take(buffer.slice(0, boundary));
            buffer = buffer.slice(boundary + 1);
        }
        if (done) break;
    }
    await take(buffer);
}
