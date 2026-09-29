import type {InitMessage, StartSyncMessage, UpdateLayoutMessage} from "./tasks/types";
import UpdateDbWorker from './tasks/wsync?worker';
import UpdateLayoutWorker from './tasks/wlayout?worker';
import {browser} from "$app/environment";
import {PUBLIC_API_PATH} from "$env/static/public";
import {getLogger} from "$lib/logger";
import {updateLayoutPort} from "$lib/stores";

const logger = getLogger();

let workers: {
    workerSync: Worker,
    workerLayout: Worker,
}

let workersChannel: MessageChannel;
let viewChannel: MessageChannel;
let syncStarted = false;

if (browser) {
    logger.info(`svebapp ${__APP_VERSION__}`);

    workersChannel = new MessageChannel();
    viewChannel = new MessageChannel();
    updateLayoutPort(viewChannel.port2);

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
        payload: [workersChannel.port2, viewChannel.port1]
    }

    workers.workerSync.postMessage(msg1, [workersChannel.port1]);
    workers.workerLayout.postMessage(msg2, [workersChannel.port2, viewChannel.port1]);

    workers.workerSync.onmessage = handleWorkerMessage;
    workers.workerLayout.onmessage = handleWorkerMessage;
}

function handleWorkerMessage(event: MessageEvent<any>) {
    const {data} = event;
    logger.info(`Task result:`, data.task, data.status);
}

export const loadFromServer = () => {
    if (!browser || syncStarted) return;
    syncStarted = true;

    const msg: StartSyncMessage = {
        task: 'start',
        payload: `${PUBLIC_API_PATH}/items`,
    };
    workers?.workerSync.postMessage(msg);
}

export const updateLayout = (screenWidth: number, rowHeight: number) => {
    const msg: UpdateLayoutMessage = {
        task: 'update',
        payload: {
            screenWidth: screenWidth,
            rowHeight: rowHeight,
        },
    }
    workers?.workerLayout.postMessage(msg);
}
