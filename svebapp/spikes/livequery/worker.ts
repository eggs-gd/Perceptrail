// Writer worker: plays the role of wsync/wlayout writing straight into IndexedDB.
import {db} from "./db";

self.onmessage = async ({data}: MessageEvent<{mode: 'batch' | 'stream', n: number, batch: number}>) => {
    const {mode, n, batch} = data;
    await db.items.clear();
    const t0 = Date.now();

    if (mode === 'batch') {
        for (let i = 0; i < n; i += batch) {
            const rows = [];
            for (let j = i; j < Math.min(n, i + batch); j++) rows.push({id: j, order: j, writtenAt: Date.now()});
            await db.items.bulkPut(rows); // one transaction per batch
        }
    } else {
        for (let i = 0; i < n; i++) {
            await db.items.put({id: i, order: i, writtenAt: Date.now()}); // one transaction per item
        }
    }
    postMessage({done: true, writeMs: Date.now() - t0});
};
