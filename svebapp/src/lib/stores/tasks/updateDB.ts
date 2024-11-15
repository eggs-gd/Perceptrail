import {Logger} from "$lib/logger";
import {itemsDb, type SvItem} from "../idb";
import {throwIfAborted} from "../types";

const logger = new Logger()

const startStream: WorkerTask<string> = async (signal: AbortSignal, api: string) => {
    const response = await fetch(api)

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

    throwIfAborted(signal)

    const processJSONChunk = async (chunk: string) => {
        buffer += chunk;

        let boundary;
        while ((boundary = buffer.indexOf("}\n")) !== -1) {
            throwIfAborted(signal)
            const jsonString = buffer.slice(0, boundary + 1);
            buffer = buffer.slice(boundary + 2);

            try {
                const data: SvItem = JSON.parse(jsonString);
                await itemsDb.items.put(data);
                logger.info("Saved to Dexie:", data);
            } catch (error) {
                logger.error("Error saving to Dexie:", error);
            }
        }
    };

    while (reader) {
        throwIfAborted(signal)

        let {done, value} = await reader.read();

        const chunk = decoder.decode(value, {stream: true});
        await processJSONChunk(chunk);

        if (done) break;
    }

    if (buffer.trim()) {
        throwIfAborted(signal)
        try {
            const data = JSON.parse(buffer);
            await itemsDb.items.put(data);
            logger.info("Rest data saved:", data);
        } catch (error) {
            logger.error("Error saving rest data:", error);
        }
    }
}

export default startStream
