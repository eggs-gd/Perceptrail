import {db, type SvItem} from './idb'
import {Idle, type WorkerMessage, WorkerTask} from "./types";

declare let self: ServiceWorkerGlobalScope

self.addEventListener('install', () => {
    self.skipWaiting().then(() => {
        console.log('Service worker installed')
    })
})
self.addEventListener('activate', event => {
    event.waitUntil(self.clients.claim())
    console.log('Service worker activated')
})

self.addEventListener('message', async (event: WorkerMessage<any>) => {
    const {task, payload} = event.data;

    switch (task) {
        case WorkerTask.UpdateDb:
            await startStream(payload.api);
            break;

    }

    postMessage(Idle);
})

async function startStream(api: string) {
    const response = await fetch(api)

    if (!response.body) {
        console.error('Streaming data is not supported');
        return {};
    }

    let stream = response.body as ReadableStream
    if (!stream) {
        console.error('Stream is empty');
        return {};
    }

    const reader = stream.getReader();
    const decoder = new TextDecoder("utf-8");
    let buffer = "";

    const processJSONChunk = async (chunk: string) => {
        buffer += chunk;

        let boundary;
        while ((boundary = buffer.indexOf("}\n")) !== -1) {
            const jsonString = buffer.slice(0, boundary + 1);
            buffer = buffer.slice(boundary + 2);

            console.log("Received chunk", jsonString);

            try {
                const data: SvItem = JSON.parse(jsonString);
                await db.items.put(data);
                //await delay(1000)
                console.log("Saved to IndexedDB:", data);
            } catch (error) {
                console.error("Error parsing JSON:", error);
            }
        }
    };

    while (reader) {
        let {done, value} = await reader.read();

        const chunk = decoder.decode(value, {stream: true});
        await processJSONChunk(chunk);

        if (done) break;
    }

    if (buffer.trim()) {
        try {
            const data = JSON.parse(buffer);
            await db.items.put(data);
            console.log("Rest data saved:", data);
        } catch (error) {
            console.error("Error saving rest data:", error);
        }
    }
}
