import type {InitMessage, OrderMessage, OrderResult, StartSyncMessage, UpdateLayoutMessage} from "./tasks/types";
import UpdateDbWorker from './tasks/wsync?worker';
import UpdateLayoutWorker from './tasks/wlayout?worker';
import {browser} from "$app/environment";
import {PUBLIC_API_PATH} from "$env/static/public";
import {getLogger} from "$lib/logger";

const logger = getLogger();

let workers: {
    workerSync: Worker,
    workerLayout: Worker,
}

let workersChannel: MessageChannel;
let syncStarted = false;

if (browser) {
    logger.info(`svebapp ${__APP_VERSION__}`);

    // wsync → wlayout; wlayout writes the layout to layoutDb, the page reads it with liveQuery
    workersChannel = new MessageChannel();

    workers = {
        workerSync: new UpdateDbWorker(),
        workerLayout: new UpdateLayoutWorker(),
    }

    const msg1: InitMessage = {
        task: "init",
        payload: [workersChannel.port1]
    }

    const msg2: InitMessage = {
        task: "init",
        payload: [workersChannel.port2]
    }

    workers.workerSync.postMessage(msg1, [workersChannel.port1]);
    workers.workerLayout.postMessage(msg2, [workersChannel.port2]);

    workers.workerSync.onmessage = handleWorkerMessage;
    workers.workerLayout.onmessage = handleWorkerMessage;
}

function handleWorkerMessage(event: MessageEvent<any>) {
    const {data} = event;
    if (data?.task === 'order') {
        const result = data as OrderResult;
        pendingOrders.get(result.id)?.(result.ok);
        pendingOrders.delete(result.id);
        return;
    }
    logger.info(`Task result:`, data.task, data.status);
}

let orderSeq = 0;
const pendingOrders = new Map<number, (ok: boolean) => void>();

export const loadFromServer = () => {
    if (!browser || syncStarted) return;
    syncStarted = true;

    const msg: StartSyncMessage = {
        task: 'start',
        payload: `${PUBLIC_API_PATH}/items`,
    };
    workers?.workerSync.postMessage(msg);
}

/** anchor: guid of the first visible item, kept in view across the relayout */
export const updateLayout = (screenWidth: number, rowHeight: number, anchor?: string) => {
    const msg: UpdateLayoutMessage = {
        task: 'update',
        payload: {screenWidth, rowHeight, anchor},
    }
    workers?.workerLayout.postMessage(msg);
}

/**
 * The sheet in a perceptor's order (GET /p/:name/order): the layout worker fetches
 * it and lays everything out again; anchor: the photo to keep in view (a relative
 * perceptor also builds its trail from it). Resolves whether it was applied: false
 * when it failed or a newer order superseded it.
 */
export const setOrder = (perceptor: string, anchor?: string): Promise<boolean> => {
    if (!workers) return Promise.resolve(false);
    const id = ++orderSeq;
    const query = anchor ? `?anchor=${encodeURIComponent(anchor)}` : '';
    const msg: OrderMessage = {
        task: 'order',
        payload: {id, url: `${PUBLIC_API_PATH}/p/${encodeURIComponent(perceptor)}/order${query}`, anchor},
    };
    return new Promise((resolve) => {
        pendingOrders.set(id, resolve);
        workers.workerLayout.postMessage(msg);
    });
}
