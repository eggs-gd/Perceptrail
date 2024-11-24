import {type Item, itemsDb} from "$lib/stores";
import {type CurrentWorkerTask, type MessageFromSync, type WorkerMessage, type WorkerTask} from "./types";
import {getLogger} from "$lib/logger";

const logger = getLogger();
let currentTask: CurrentWorkerTask = null;

let msgPrt: MessagePort;

self.onmessage = async function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    if (task === 'init') {
        msgPrt = payload;
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
            try {
                await updateDbStreamed(controller.signal, apiPath);
                postMessage({status: "completed"});
            } catch (error) {
                postMessage({status: "error"});
                logger.error("Error in UpdateDb task:", error);
            } finally {
                itemsDb.items.hook.creating.unsubscribe(hookCreate);
                itemsDb.items.hook.updating.unsubscribe(hookUpdate);
                currentTask = null;
            }
        })(),
    };
}

function hookCreate(key: string, item: Item) {
    const msg: MessageFromSync = {added: true, item: item};
    msgPrt.postMessage(msg);
}

function hookUpdate(mods: Object, key: string, item: Item) {
    if (mods.hasOwnProperty("width") || mods.hasOwnProperty("height")) {
        const msg: MessageFromSync = {added: false, item: item};
        msgPrt.postMessage(msg);
    }
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
