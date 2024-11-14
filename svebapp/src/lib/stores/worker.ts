import {Idle, type WorkerMessage, WorkerTask} from "./types";
import {updateDbStreamed} from "$lib/stores/tasks";

self.onmessage = async function(event:WorkerMessage<any>) {
    const {task, payload} = event.data;

    switch (task) {
        case WorkerTask.UpdateDb:
            await updateDbStreamed(payload.api);
            break;
    }

    postMessage(Idle);
}
