import {itemsDb, type SvItem} from "../idb";
import {type UpdateDbPayload} from "../types";
import type {WorkerTask} from "./types";
import {getLogger} from "$lib/logger/logger";

const logger = getLogger()

export const updateDbStreamed: WorkerTask<UpdateDbPayload> = async (signal: AbortSignal, payload: UpdateDbPayload) => {
    const response = await fetch(payload.api)

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
                const data: SvItem = JSON.parse(jsonString);
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
