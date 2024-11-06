import type {PageLoad} from './$types';
import {db, type SvItem} from "./idb";


export const load: PageLoad = async ({data}: { data: { streamUrl: string } | null }) => {
    if (!data) {
        console.error('Streaming url not found');
        return {};
    }
    const response = await fetch(data?.streamUrl)

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

    return {}
}
