import {Idle, type UpdateDbMessageData, type UpdateLayoutMessageData, type WorkerMessageData} from "./types";
import Worker from './worker?worker'
import {browser} from "$app/environment";
import {PUBLIC_API_PATH} from "$env/static/public";
import {Logger} from "$lib/logger";


const logger = new Logger()

const workerPoolSize = 4;
const workers: Worker[] = [];
const availableWorkers: Worker[] = [];
const taskQueue: WorkerMessageData<any>[] = [];


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
        const nextTask = taskQueue.shift();
        const worker = event.target as Worker;
        nextTask ? worker.postMessage(nextTask) : availableWorkers.push(worker);
    } else {
        logger.info(`Task ${data.taskId} result:`, data.result);
    }
}

function enqueueTask<T>(msg: WorkerMessageData<T>) {
    if (availableWorkers.length > 0) {
        const worker = availableWorkers.pop() as Worker;
        worker.postMessage(msg);
    } else {
        taskQueue.push(msg);
        logger.info('Worker pool is empty, msg added to queue', WorkerTaskType[msg.task]);
    }
}

let currentLoadingController: AbortController | null = null;
export const loadFromServer = () => {
    if (currentLoadingController) { // postpone new instead of abort current
        currentLoadingController.abort();
    }

    const controller = new AbortController();
    currentLoadingController = controller;

    const msg: UpdateDbMessageData = {
        task: WorkerTaskType.UpdateDb,
        signal: controller.signal,
        payload: {
            api: `${PUBLIC_API_PATH}/items`
        }
    }

    enqueueTask(msg)
}

let currentLayoutController: AbortController | null = null;
let currentWidth: number;
export const updateLayout = (width: number /*todo add sorting strategy*/) => {
    if (currentWidth == width /*todo <and the same sorting>*/) {
        return;
    }

    if (currentLayoutController) {
        currentLayoutController.abort();
    }

    const controller = new AbortController();
    currentLayoutController = controller;

    const msg: UpdateLayoutMessageData = {
        task: WorkerTaskType.UpdateLayout,
        signal: controller.signal,
        payload: {
            width: width
        }
    }

    enqueueTask(msg)
}
