import 'fake-indexeddb/auto';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {type Item, itemsDb} from '#lib/stores/index.js';
import {open, store} from './stream';

const item = (guid: string) => ({guid, date: '2024-07-01T10:00:00Z'}) as unknown as Item;

// An NDJSON body, in chunks as the network may cut it (a line across two)
const ndjson = (lines: object[], epoch = 'db.7', cut?: number) => {
    const text = lines.map((l) => JSON.stringify(l)).join('\n');
    const chunks = cut ? [text.slice(0, cut), text.slice(cut)] : [text];
    const body = new ReadableStream<Uint8Array>({
        start(c) {
            for (const chunk of chunks) c.enqueue(new TextEncoder().encode(chunk));
            c.close();
        },
    });
    return new Response(body, {headers: {'X-Sync-Epoch': epoch}});
};

const signal = new AbortController().signal;

beforeEach(async () => {
    await itemsDb.items.clear();
});
afterEach(() => {
    vi.unstubAllGlobals();
});

describe('open', () => {
    it('no copy yet: everything', async () => {
        const fetch = vi.fn(async () => ndjson([]));
        vi.stubGlobal('fetch', fetch);
        const {epoch, delta} = await open(signal, '/api/items');
        expect([epoch, delta]).toEqual(['db.7', false]);
        expect(fetch).toHaveBeenCalledWith('/api/items', {signal});
    });

    it('the same database and contract: the delta since the cursor', async () => {
        const fetch = vi.fn(async () => ndjson([]));
        vi.stubGlobal('fetch', fetch);
        const {delta} = await open(signal, '/api/items', {cursor: '2024-07-01 10:00:00+03:00', epoch: 'db.7'});
        expect(delta).toBe(true);
        expect(fetch).toHaveBeenCalledWith('/api/items?since=2024-07-01%2010%3A00%3A00%2B03%3A00', {signal});
    });

    it('another epoch: the delta dropped, everything asked for', async () => {
        const first = ndjson([], 'db.8');
        const cancel = vi.spyOn(first.body!, 'cancel');
        const fetch = vi.fn().mockResolvedValueOnce(first).mockResolvedValueOnce(ndjson([], 'db.8'));
        vi.stubGlobal('fetch', fetch);
        const {epoch, delta} = await open(signal, '/api/items', {cursor: 'c', epoch: 'db.7'});
        expect([epoch, delta]).toEqual(['db.8', false]);
        expect(cancel).toHaveBeenCalled();
        expect(fetch).toHaveBeenLastCalledWith('/api/items', {signal});
    });
});

describe('store', () => {
    it('items in, a removed one out, the cursor and the total read — a line cut by the network too', async () => {
        await itemsDb.items.put(item('old'));
        const lines = [item('a'), item('b'), {guid: 'old', removed: true}, {cursor: 'c1', total: 2}];
        const got = await store(signal, ndjson(lines, 'db.7', 25));
        expect(got).toEqual({changed: 3, cursor: 'c1', total: 2});
        expect((await itemsDb.items.toCollection().primaryKeys()).sort()).toEqual(['a', 'b']);
    });

    it('everything over the copy: what the stream did not bring is gone', async () => {
        await itemsDb.items.bulkPut([item('a'), item('stale')]);
        const got = await store(signal, ndjson([item('a'), {cursor: 'c2'}]), new Set());
        expect(got).toEqual({changed: 2, cursor: 'c2', total: -1});
        expect(await itemsDb.items.toCollection().primaryKeys()).toEqual(['a']);
    });

    it('cut short (no cursor): nothing deleted', async () => {
        await itemsDb.items.put(item('stale'));
        const got = await store(signal, ndjson([item('a')]), new Set());
        expect(got.cursor).toBe('');
        expect((await itemsDb.items.toCollection().primaryKeys()).sort()).toEqual(['a', 'stale']);
    });

    it('a line that is not JSON fails the read', async () => {
        const body = new Response('{"guid":"a"}\nnot json\n');
        await expect(store(signal, body)).rejects.toThrow();
    });
});
