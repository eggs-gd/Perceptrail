import {type UpdateDbMessageData, type UpdateLayoutMessageData, type WorkerMessageData} from "./types";
import Worker from './worker?worker'
import {browser} from "$app/environment";
import {PUBLIC_API_PATH} from "$env/static/public";
import {getLogger} from "$lib/logger";
import {WorkerTaskType} from "./tasks/types";
import type {Item} from "$lib/stores";


const logger = getLogger()

const workerPoolSize = 4;
const workers: Worker[] = [];
const availableWorkers: Worker[] = [];
const taskQueue: WorkerMessageData<any>[] = [];

const activeTasks = new Map<WorkerTaskType, Worker>();

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

    if (data.idle) {
        processFreeWorker(event.target as Worker)
        return;
    }

    switch (data.task) {
        case WorkerTaskType.UpdateDb:
            processUpdateDbMessage(data)
            break;
        default:
            logger.info(`Task result:`, WorkerTaskType[data.task], data.status);
    }
}

function processFreeWorker(worker: Worker) {
    const nextTask = taskQueue.shift();

    if (nextTask) {
        worker.postMessage(nextTask)
    } else {
        availableWorkers.push(worker);
    }
}

function processUpdateDbMessage(data: { task: WorkerTaskType.UpdateDb, status: string, payload: Item }) {
    if (activeTasks.has(WorkerTaskType.UpdateLayout)) {
        const worker = activeTasks.get(data.task) as Worker
        logger.debug('got update from hook', data);
        const msg: UpdateLayoutMessageData = {
            aborted: false,
            task: WorkerTaskType.UpdateLayout,
            payload: {
                screenWidth: currentScreenWidth,
                rowHeight: currentRowHeight,
                item: data.payload,
                added: data.status === "add_item"
            }
        }
        worker.postMessage(msg);
    }
}

function enqueueTask<T>(msg: WorkerMessageData<T>) {
    logger.info('Enqueue task', msg);

    if (availableWorkers.length > 0) {
        const worker = availableWorkers.pop() as Worker;
        activeTasks.set(msg.task, worker);
        worker.postMessage(msg);
    } else {
        taskQueue.push(msg);
        logger.warn('Worker pool is empty, msg added to queue', WorkerTaskType[msg.task], workers.length, availableWorkers.length, activeTasks.keys());
    }
}

function cancelTask(taskType: WorkerTaskType) {
    logger.info('Cancel task', WorkerTaskType[taskType]);

    if (!activeTasks.has(taskType)) {
        logger.info("Don't have running task", WorkerTaskType[taskType])
    } else {
        const worker = activeTasks.get(taskType) as Worker;
        worker.postMessage({task: taskType, aborted: true});
    }
}

let currentLoadingController: AbortController | null = null;
export const loadFromServer = () => {
    if (currentLoadingController) { // todo postpone new instead of abort current
        currentLoadingController.abort();
    }

    currentLoadingController = new AbortController();
    currentLoadingController.signal.addEventListener('abort', () => {
        cancelTask(WorkerTaskType.UpdateDb)
    })

    const msg: UpdateDbMessageData = {
        aborted: false,
        task: WorkerTaskType.UpdateDb,
        payload: {
            api: `${PUBLIC_API_PATH}/items`
        }
    }

    enqueueTask(msg)
}

let currentLayoutController: AbortController | null = null;
let currentScreenWidth: number;
let currentRowHeight: number;
export const updateLayout = (screenWidth: number, rowHeight: number) => {
    if (currentScreenWidth == screenWidth &&
        currentRowHeight == rowHeight /*todo <and the same sorting>*/) {
        return;
    }

    currentScreenWidth = screenWidth;
    currentRowHeight = rowHeight;

    if (currentLayoutController) {
        currentLayoutController.abort();
    }

    currentLayoutController = new AbortController();
    currentLayoutController.signal.addEventListener('abort', () => {
        cancelTask(WorkerTaskType.UpdateLayout)
    })

    const msg: UpdateLayoutMessageData = {
        aborted: false,
        task: WorkerTaskType.UpdateLayout,
        payload: {
            screenWidth: screenWidth,
            rowHeight: rowHeight,
        }
    }

    enqueueTask(msg)
}
