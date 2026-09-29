// liveQuery spike: do writes from a worker reach liveQuery subscriptions on the page,
// how fast, and does a window subscription ignore writes outside the window?
import {liveQuery} from "dexie";
import {db} from "./db";
import Writer from "./worker.ts?worker";

const out = document.getElementById('out')!;
const log: string[] = [];
const print = (s: string) => { log.push(s); out.textContent = log.join('\n'); };

function percentile(xs: number[], p: number) {
    if (!xs.length) return NaN;
    const s = [...xs].sort((a, b) => a - b);
    return s[Math.min(s.length - 1, Math.floor(p * s.length))];
}

async function scenario(name: string, mode: 'batch' | 'stream', n: number, batch = 1) {
    await db.items.clear();
    await new Promise(r => setTimeout(r, 300)); // let the clear's notifications settle

    const stats = {countEmits: 0, finalCount: 0, lastEmits: 0, latencies: [] as number[],
        windowEmits: 0, windowEmitsAfterFilled: 0, window2Emits: 0};

    // 1) whole-table count
    const countSub = liveQuery(() => db.items.count()).subscribe(c => { stats.countEmits++; stats.finalCount = c; });

    // 2) newest item → latency from the worker's write to the page seeing it
    const lastSub = liveQuery(() => db.items.orderBy('id').last()).subscribe(it => {
        stats.lastEmits++;
        if (it) stats.latencies.push(Date.now() - it.writtenAt);
    });

    // 3) a "visible window" of 100 items at the beginning: once filled, later writes
    //    (outside the window) should not re-run it
    let windowFilled = false;
    const windowSub = liveQuery(() => db.items.where('order').between(0, 100).toArray()).subscribe(rows => {
        stats.windowEmits++;
        if (windowFilled) stats.windowEmitsAfterFilled++;
        if (rows.length === 100) windowFilled = true;
    });

    // 4) a window that the stream reaches later (like scrolling to where data arrives)
    const mid = Math.floor(n * 0.75);
    const window2Sub = liveQuery(() => db.items.where('order').between(mid, mid + 100).toArray())
        .subscribe(() => { stats.window2Emits++; });

    const writer = new Writer();
    const done = await new Promise<{writeMs: number}>(resolve => {
        writer.onmessage = e => resolve(e.data);
        writer.postMessage({mode, n, batch});
    });
    await new Promise(r => setTimeout(r, 500)); // late notifications
    writer.terminate();
    [countSub, lastSub, windowSub, window2Sub].forEach(s => s.unsubscribe());

    const result = {
        scenario: name,
        writes: mode === 'batch' ? Math.ceil(n / batch) + ' tx × ' + batch : n + ' tx × 1',
        writeMs: done.writeMs,
        finalCount: stats.finalCount, ok: stats.finalCount === n,
        countEmits: stats.countEmits,
        latencyMs: {p50: percentile(stats.latencies, .5), p95: percentile(stats.latencies, .95), max: Math.max(...stats.latencies)},
        windowEmits: stats.windowEmits, windowEmitsAfterFilled: stats.windowEmitsAfterFilled,
        window2Emits: stats.window2Emits,
    };
    print(JSON.stringify(result));
    return result;
}

(async () => {
    const results = [
        await scenario('batch 5000 × 100', 'batch', 5000, 100),
        await scenario('stream 2000 × 1', 'stream', 2000),
    ];
    (window as any).__spike = results;
    print('DONE');
})();
