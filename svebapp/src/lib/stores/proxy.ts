import {Idle, type UpdateMessageData, WorkerTask} from "./types";
import Worker from './worker?worker'
import {browser} from "$app/environment";
import {PUBLIC_API_PATH} from "$env/static/public";


const workerPoolSize = 4;
const workers: Worker[] = [];
const availableWorkers: Worker[] = [];

if (browser) {
    for (let i = 0; i < workerPoolSize; i++) {
        const worker = new Worker();
        worker.onmessage = handleWorkerMessage;
        workers.push(worker);
        availableWorkers.push(worker);
    }
}

function handleWorkerMessage(event: MessageEvent<any>) {
    const {data} = event;

    if (data === Idle) {
        availableWorkers.push(event.target as Worker);
    } else {
        console.log(`Task ${data.taskId} result:`, data.result);
    }
}

export const loadFromServer = () => {
    const msg: UpdateMessageData = {
        task: WorkerTask.UpdateDb,
        payload: {
            api: `${PUBLIC_API_PATH}/items`
        }
    }

    if (availableWorkers.length > 0) {
        const worker = availableWorkers.pop() as Worker;
        worker.postMessage(msg);
    } else {
        // todo add queue
        console.log('Worker pool is empty');
    }
}
