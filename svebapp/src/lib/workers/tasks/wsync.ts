import {type Item, itemsDb} from "$lib/stores";
import {type CurrentWorkerTask, type MessageFromSync, type WorkerMessage, type WorkerTask} from "./types";
import {getLogger} from "$lib/logger";

const logger = getLogger();
let currentTask: CurrentWorkerTask = null;

let updatesPort: MessagePort;

self.onmessage = async function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    if (task === 'init') {
        updatesPort = payload[0];
        logger.debug('Inited');
    } else if (task === 'start' && !currentTask) {
        currentTask = startNewTask(payload);
        logger.debug('Started')
    } else if (task === 'start' && currentTask) {
        postMessage({status: "already_running"});
    } else if (task === 'update' && currentTask) {
        //todo add to queue
        //todo add logic to manage queue to keep only one last request
    }
};

function startNewTask(apiPath: string) {
    const controller = new AbortController();
    return {
        controller,
        promise: (async () => {
            itemsDb.items.hook.creating.subscribe(hookCreate);
            itemsDb.items.hook.updating.subscribe(hookUpdate);
            itemsDb.items.hook.deleting.subscribe(hookDelete);
            try {
                updatesPort.postMessage({action: 'sync-start'});
                await updateDbStreamed(controller.signal, apiPath);
                updatesPort.postMessage({action: 'sync-done'});
                postMessage({status: "completed"});
            } catch (error) {
                postMessage({status: "error"});
                logger.error("Error in UpdateDb task:", error);
            } finally {
                itemsDb.items.hook.creating.unsubscribe(hookCreate);
                itemsDb.items.hook.updating.unsubscribe(hookUpdate);
                itemsDb.items.hook.deleting.unsubscribe(hookDelete);
                currentTask = null;
            }
        })(),
    };
}

function hookCreate(key: string, item: Item) {
    const msg: MessageFromSync = {action: "create", item: item};
    updatesPort.postMessage(msg);
}

function hookUpdate(mods: Object, key: string, item: Item) {
    if (mods.hasOwnProperty("width") || mods.hasOwnProperty("height")) {
        // Dexie passes the pre-update object: merge mods so the new size gets through
        const msg: MessageFromSync = {action: "update", item: {...item, ...mods}};
        updatesPort.postMessage(msg);
    }
}

function hookDelete(key: string, item: Item) {
    const msg: MessageFromSync = {action: "delete", item: item};
    updatesPort.postMessage(msg);
}

const updateDbStreamed: WorkerTask<string> = async (signal: AbortSignal, payload: string) => {
    const response = await fetch(payload)

    if (!response.body) {
        logger.error('Streaming data is not supported');
        return;
    }

    let stream = response.body as ReadableStream
    if (!stream) {
        logger.error('Stream is empty');
        return;
    }

    const reader = stream.getReader();
    const decoder = new TextDecoder("utf-8");
    let buffer = "";

    signal.throwIfAborted()

    const processJSONChunk = async (chunk: string) => {
        buffer += chunk;

        let boundary;
        while ((boundary = buffer.indexOf("}\n")) !== -1) {
            signal.throwIfAborted()
            const jsonString = buffer.slice(0, boundary + 1);
            buffer = buffer.slice(boundary + 2);

            try {
                const data: Item = JSON.parse(jsonString);
                await itemsDb.items.put(data);
                //logger.debug("Saved to Dexie:", data);
            } catch (error) {
                logger.error("Error saving to Dexie:", error);
            }
        }
    };

    while (reader) {
        signal.throwIfAborted()

        let {done, value} = await reader.read();

        const chunk = decoder.decode(value, {stream: true});
        await processJSONChunk(chunk);

        if (done) break;
    }

    if (buffer.trim()) {
        signal.throwIfAborted()
        try {
            const data = JSON.parse(buffer);
            await itemsDb.items.put(data);
            logger.info("Rest data saved:", data);
        } catch (error) {
            logger.error("Error saving rest data:", error);
        }
    }
}
