import {Idle, type WorkerMessage} from "./types";
import {updateDbStreamed, updateLayoutStreamed} from "$lib/stores/tasks";
import {Logger} from "$lib/logger";

const logger = new Logger()

self.onmessage = async function (event: WorkerMessage<any>) {
    const {task, signal, payload} = event.data;

    try {
        switch (task) {
            case WorkerTaskType.UpdateDb:
                await updateDbStreamed(signal, payload.api);
                break;

            case WorkerTaskType.UpdateLayout:
                await updateLayoutStreamed(signal, payload.width);
                break;
        }
    } catch (error) {
        logger.error(`Error in worker task ${task}:`, error);
    }
    postMessage(Idle);
}
