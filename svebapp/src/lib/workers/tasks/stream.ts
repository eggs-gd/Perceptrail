import {type Item, itemsDb} from "#lib/stores/index.js";

// The server's item stream as the sync reads it (GET /items, NDJSON): which one to
// ask for, and its lines into the copy. The sync worker (wsync.ts) runs it; it is
// apart from the worker so it can be read — and tested — without one.

export type SyncLine = (Item & {removed?: boolean}) | {cursor: string, total?: number};

/**
 * open: the response to read — the delta since state's cursor while the server is
 * the same database and contract (its epoch), else everything; no state: everything
 */
export async function open(signal: AbortSignal, apiPath: string, state?: {cursor: string, epoch: string}):
    Promise<{response: Response, epoch: string, delta: boolean}> {
    if (state) {
        const response = await fetch(`${apiPath}?since=${encodeURIComponent(state.cursor)}`, {signal});
        const epoch = response.headers.get('X-Sync-Epoch') ?? '';
        if (epoch === state.epoch) return {response, epoch, delta: true};
        // Another database or another contract: our copy is not a base for its delta
        await response.body?.cancel();
    }
    const response = await fetch(apiPath, {signal});
    return {response, epoch: response.headers.get('X-Sync-Epoch') ?? '', delta: false};
}

/**
 * store: the stream's items into the copy, a removed one out; seen: every guid it
 * brings (then what it did not bring goes, once the stream ended with its cursor)
 */
export async function store(signal: AbortSignal, response: Response, seen?: Set<string>):
    Promise<{changed: number, cursor: string, total: number}> {
    let changed = 0;
    let cursor = '';
    let total = -1;
    await readLines(signal, response, async (line) => {
        if ('cursor' in line) {
            cursor = line.cursor;
            total = line.total ?? -1;
            return;
        }
        if (line.removed) await itemsDb.items.delete(line.guid);
        else await itemsDb.items.put(line as Item);
        seen?.add(line.guid);
        changed++;
    });
    if (cursor && seen) {
        const gone = (await itemsDb.items.toCollection().primaryKeys()).filter((g) => !seen.has(g));
        await itemsDb.items.bulkDelete(gone);
        changed += gone.length;
    }
    return {changed, cursor, total};
}

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
