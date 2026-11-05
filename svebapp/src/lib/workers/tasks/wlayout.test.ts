import 'fake-indexeddb/auto';
import {afterAll, afterEach, describe, expect, it, vi} from 'vitest';
import {
    type Item,
    itemsDb,
    LAYOUT_META_KEY,
    LAYOUT_SECTIONS_KEY,
    LAYOUT_SIZE_KEY,
    type LayoutMeta,
    type LayoutSections,
    type LayoutSize,
    useLayoutDb,
} from '#lib/stores/index.js';
import {ITEMS_CHANNEL, type MessageFromSync, type OrderEntry, type OrderResult} from './types';

// The layout worker as its page and the sync see it: messages in (the page's
// init/update/order, the sync's channel), the layout out (the page's layoutDb, the
// order's answer). Each test loads the module afresh: its state is the module's.

// A photo w×h; at rowHeight 100 its natural width is w (h = 100)
const photo = (guid: string, w: number, h = 100, date = '2024-07-01T10:00:00Z') =>
    ({guid, width: w, height: h, date: new Date(date)}) as unknown as Item;

const W = 1000, H = 100, GUTTER = 8;

const sync = new BroadcastChannel(ITEMS_CHANNEL);
let n = 0;
let layout: ReturnType<typeof useLayoutDb>;
let answers: OrderResult[];

/** Loads the worker over the kept items; returns its page's post */
async function start(kept: Item[] = []) {
    await itemsDb.items.clear();
    await itemsDb.orders.clear();
    await itemsDb.items.bulkPut(kept);
    answers = [];
    vi.stubGlobal('postMessage', (m: OrderResult) => answers.push(m));
    vi.resetModules();
    await import('./wlayout');
    const post = (task: string, payload?: unknown) =>
        (globalThis.onmessage as unknown as (m: {data: unknown}) => void)({data: {task, payload}});
    const name = `layout-test-${++n}`;
    post('init', {layoutDb: name});
    layout = useLayoutDb(name);
    return post;
}

const send = (data: MessageFromSync) => sync.postMessage(data);
const rows = () => layout.items.orderBy('order').toArray();
const meta = async () => (await layout.meta.get(LAYOUT_META_KEY)) as LayoutMeta | undefined;
const size = async () => (await layout.meta.get(LAYOUT_SIZE_KEY)) as LayoutSize | undefined;
const marks = async () => ((await layout.meta.get(LAYOUT_SECTIONS_KEY)) as LayoutSections | undefined)?.marks;
const guids = async () => (await rows()).map((r) => r.guid);

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
/**
 * Waits for a laid-out sheet that stays (no relayout for longer than the worker's
 * throttle) and returns its rev: how many relayouts it took is the worker's business
 */
async function settle(): Promise<number> {
    let rev = (await meta())?.rev;
    for (let tries = 0; tries < 40; tries++) {
        await sleep(60);
        const now = (await meta())?.rev;
        if (now !== undefined && now === rev) return now;
        rev = now;
    }
    throw new Error('the layout did not settle');
}

/** A server's /p/:name/order */
const ndjson = (entries: OrderEntry[]) => new Response(entries.map((e) => JSON.stringify(e)).join('\n'));

afterEach(() => {
    vi.unstubAllGlobals();
});
afterAll(() => {
    sync.close();
});

describe('rows', () => {
    it('a row that overflows by less than half of the next photo closes without it, stretched to the width', async () => {
        const post = await start([photo('a', 400), photo('b', 400), photo('c', 300)]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        // (1000 - 8) / 800 of the row height: both fill the width exactly
        const [a, b, c] = await rows();
        expect([a.x, a.y, a.w, a.h]).toEqual([0, 0, 496, 124]);
        expect([b.x, b.y, b.w, b.h]).toEqual([504, 0, 496, 124]);
        expect(b.x + b.w).toBe(W);
        expect([a.row, b.row]).toEqual([0, 0]);
        // The last row stays open: the target height, left-aligned
        expect([c.x, c.y, c.w, c.h, c.row]).toEqual([0, 124 + GUTTER, 300, H, 1]);
        expect(c.bottom).toBe(c.y + c.h);
        expect((await meta())?.height).toBe(124 + GUTTER + H);
        expect(await size()).toMatchObject({rev: (await meta())?.rev, count: 3, height: 124 + GUTTER + H});
    });

    it('a row that overflows by more takes the photo and shrinks', async () => {
        const post = await start([photo('a', 400), photo('b', 400), photo('c', 400)]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        const all = await rows();
        expect(all.map((r) => [r.row, r.x, r.h])).toEqual([[0, 0, 82], [0, 336, 82], [0, 672, 82]]);
        expect(all[0].scale).toBe(0.82);
        // No open row: the height ends at the last row, without its gutter
        expect((await meta())?.height).toBe(82);
    });

    it('a photo wider than the screen is a row of its own', async () => {
        // Equal dates: the kept items come in guid order
        const post = await start([photo('a-wide', 1600), photo('b-next', 300)]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        const [wide, next] = await rows();
        expect([wide.w, wide.h, wide.row]).toEqual([W, 62.5, 0]);
        expect([next.y, next.row]).toEqual([62.5 + GUTTER, 1]);
    });

    it('nothing is laid out before the viewport; the same viewport again changes nothing', async () => {
        const post = await start([photo('a', 400)]);
        await sleep(100);
        expect(await meta()).toBeUndefined();

        post('update', {screenWidth: W, rowHeight: H});
        const rev = await settle();
        post('update', {screenWidth: W, rowHeight: H});
        await sleep(100);
        expect((await meta())?.rev).toBe(rev);

        post('update', {screenWidth: 600, rowHeight: H});
        await vi.waitFor(async () => expect((await meta())?.width).toBe(600));
        expect((await meta())?.rev).toBeGreaterThan(rev);
    });

    it('the anchor photo is reported where it moved', async () => {
        const post = await start([photo('a', 400), photo('b', 400), photo('c', 300)]);
        post('update', {screenWidth: W, rowHeight: H, anchor: 'c'});
        await settle();
        expect((await meta())?.anchor).toEqual({guid: 'c', y: 132, h: H});
    });

    it('the kept items are the sheet, newest first, before the sync says anything', async () => {
        const post = await start([
            photo('old', 300, 100, '2020-01-01T00:00:00Z'),
            photo('new', 300, 100, '2024-01-01T00:00:00Z'),
        ]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();
        expect(await guids()).toEqual(['new', 'old']);
    });
});

describe('the sync', () => {
    it('streamed photos are appended in batches, without a relayout', async () => {
        const post = await start();
        post('update', {screenWidth: W, rowHeight: H});
        const rev = await settle();

        send({action: 'create', item: photo('a', 400)});
        send({action: 'create', item: photo('b', 400)});
        send({action: 'create', item: photo('c', 300)});
        await vi.waitFor(async () => expect(await size()).toMatchObject({count: 3, height: 232}));
        expect(await guids()).toEqual(['a', 'b', 'c']);
        expect((await rows())[0].w).toBe(496); // the first row closed as it filled
        expect((await meta())?.rev).toBe(rev);
    });

    it('photos that come while a relayout is pending are in it', async () => {
        const post = await start();
        post('update', {screenWidth: W, rowHeight: H});
        send({action: 'create', item: photo('a', 400)});
        send({action: 'create', item: photo('b', 400)});
        await vi.waitFor(async () => expect(await guids()).toEqual(['a', 'b']));
    });

    it('a new size of a photo moves everything after it; a new preview patches its row in place', async () => {
        const post = await start([photo('a', 400), photo('b', 400), photo('c', 300)]);
        post('update', {screenWidth: W, rowHeight: H});
        const rev = await settle();

        send({action: 'update', item: {...photo('a', 400), previewColor: '#123456'}});
        await vi.waitFor(async () => expect((await rows())[0].previewColor).toBe('#123456'));
        expect((await rows())[0]).toMatchObject({x: 0, y: 0, w: 496, h: 124});
        expect((await meta())?.rev).toBe(rev);

        send({action: 'update', item: photo('a', 200)});
        await vi.waitFor(async () => expect((await rows()).map((r) => r.row)).toEqual([0, 0, 0]));
        expect((await meta())?.rev).toBeGreaterThan(rev);
    });

    it('rows streamed for the old sheet do not land after a relayout', async () => {
        const post = await start();
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        send({action: 'create', item: photo('a', 400)});
        send({action: 'create', item: photo('b', 400)});
        send({action: 'sync-start'});
        await settle();
        await sleep(100);
        expect(await rows()).toEqual([]);
    });

    it('a deleted photo leaves the sheet; a new sync starts it empty', async () => {
        const post = await start([photo('a', 400), photo('b', 400)]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        send({action: 'delete', item: photo('a', 400)});
        await vi.waitFor(async () => expect(await guids()).toEqual(['b']));

        send({action: 'sync-start'});
        await vi.waitFor(async () => expect(await rows()).toEqual([]));
        expect((await meta())?.height).toBe(0);
    });
});

describe('the order', () => {
    const view = (id: number, anchor?: string) => ({id, view: 'place', url: `/p/place/order?${id}`, anchor});

    it("a perceptor's order sorts the sheet and marks its sections; it is kept for the view", async () => {
        vi.stubGlobal('fetch', async () => ndjson([
            {guid: 'c', sections: [{level: 0, label: 'Kyiv'}]},
            {guid: 'a'},
            {guid: 'b', sections: [{level: 0, label: 'Lviv'}, {level: 1, label: 'Centre'}]},
        ]));
        const post = await start([photo('a', 400), photo('b', 400), photo('c', 300)]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        post('order', view(1, 'b'));
        await vi.waitFor(() => expect(answers).toEqual([{task: 'order', id: 1, ok: true}]));
        await vi.waitFor(async () => expect(await guids()).toEqual(['c', 'a', 'b']));
        const b = (await rows())[2];
        expect((await meta())?.anchor).toEqual({guid: 'b', y: b.y, h: b.h});
        expect(await marks()).toEqual([
            {level: 0, label: 'Kyiv', guid: 'c', order: 0, y: 0},
            {level: 0, label: 'Lviv', guid: 'b', order: 2, y: b.y},
            {level: 1, label: 'Centre', guid: 'b', order: 2, y: b.y},
        ]);
        await vi.waitFor(async () => expect((await itemsDb.orders.get('order:place'))?.entries).toHaveLength(3));
    });

    it('a photo the order puts earlier than the end of the sheet goes in its place', async () => {
        vi.stubGlobal('fetch', async () => ndjson([{guid: 'late', sections: [{level: 0, label: 'First'}]}, {guid: 'a'}]));
        const post = await start([photo('a', 300), photo('b', 300)]);
        post('update', {screenWidth: W, rowHeight: H});
        post('order', view(1));
        await vi.waitFor(() => expect(answers).toHaveLength(1));

        send({action: 'create', item: photo('late', 300)});
        await vi.waitFor(async () => expect(await guids()).toEqual(['late', 'a', 'b']));
        await vi.waitFor(async () => expect((await marks())?.map((m) => m.guid)).toEqual(['late']));
    });

    it('a streamed photo that starts a section marks it', async () => {
        vi.stubGlobal('fetch', async () => ndjson([{guid: 'a'}, {guid: 'b', sections: [{level: 0, label: '2024'}]}]));
        const post = await start();
        post('update', {screenWidth: W, rowHeight: H});
        post('order', view(1));
        await vi.waitFor(() => expect(answers).toHaveLength(1));
        await settle();

        send({action: 'create', item: photo('a', 300)});
        send({action: 'create', item: photo('b', 300)});
        await vi.waitFor(async () => expect(await marks()).toEqual([{level: 0, label: '2024', guid: 'b', order: 1, y: 0}]));
    });

    it("the view's kept order is used at once; the same order from the server changes nothing", async () => {
        const entries = [{guid: 'b'}, {guid: 'a'}];
        let release!: () => void;
        const answered = new Promise<void>((r) => (release = r));
        vi.stubGlobal('fetch', async () => {
            await answered;
            return ndjson(entries);
        });
        const post = await start([photo('a', 300), photo('b', 300)]);
        await itemsDb.orders.put({key: 'order:place', entries});
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        post('order', view(1));
        await vi.waitFor(async () => expect(await guids()).toEqual(['b', 'a']));
        const rev = await settle();
        release();
        await vi.waitFor(() => expect(answers).toEqual([{task: 'order', id: 1, ok: true}]));
        await sleep(100);
        expect((await meta())?.rev).toBe(rev);
    });

    it('a server failure keeps the kept order; with nothing kept the switch fails', async () => {
        vi.stubGlobal('fetch', async () => new Response('', {status: 500}));
        // Expected here: logged, not printed
        const error = vi.spyOn(console, 'error').mockImplementation(() => {});
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
        const post = await start([photo('a', 300), photo('b', 300)]);
        post('update', {screenWidth: W, rowHeight: H});
        await settle();

        post('order', view(1));
        await vi.waitFor(() => expect(answers).toEqual([{task: 'order', id: 1, ok: false}]));
        expect(await guids()).toEqual(['a', 'b']);
        expect(error).toHaveBeenCalledWith(expect.any(String), expect.any(String), 'order failed', expect.any(Error));

        await itemsDb.orders.put({key: 'order:place', entries: [{guid: 'b'}, {guid: 'a'}]});
        post('order', view(2));
        await vi.waitFor(() => expect(answers.at(-1)).toEqual({task: 'order', id: 2, ok: true}));
        await vi.waitFor(async () => expect(await guids()).toEqual(['b', 'a']));
        expect(warn).toHaveBeenCalledWith(expect.any(String), expect.any(String), 'order not refreshed, the kept one stays', expect.any(Error));
        error.mockRestore();
        warn.mockRestore();
    });

    it('a newer order aborts the one being fetched', async () => {
        vi.stubGlobal('fetch', (url: string, {signal}: {signal: AbortSignal}) => url.endsWith('?1')
            ? new Promise((_, reject) => signal.addEventListener('abort', () => reject(signal.reason)))
            : Promise.resolve(ndjson([{guid: 'b'}, {guid: 'a'}])));
        const post = await start([photo('a', 300), photo('b', 300)]);
        post('update', {screenWidth: W, rowHeight: H});

        post('order', view(1));
        post('order', view(2));
        await vi.waitFor(() => expect(answers).toHaveLength(2));
        expect(answers).toEqual(expect.arrayContaining([
            {task: 'order', id: 1, ok: false},
            {task: 'order', id: 2, ok: true},
        ]));
        await vi.waitFor(async () => expect(await guids()).toEqual(['b', 'a']));
    });
});

describe('the mode', () => {
    it('debug turns the debug lines on', async () => {
        const debug = vi.spyOn(console, 'debug').mockImplementation(() => {});
        const post = await start([photo('a', 300)]);
        post('mode', 'debug');
        post('update', {screenWidth: W, rowHeight: H});
        await settle();
        expect(debug).toHaveBeenCalled();
        post('mode', 'prod');
        debug.mockRestore();
    });
});
