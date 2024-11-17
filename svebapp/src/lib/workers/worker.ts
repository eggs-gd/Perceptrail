import {
    Idle,
    type UpdateDbPayload,
    type UpdateLayoutPayload,
    type WorkerMessage,
    type WorkerMessageData
} from "./types";
import {layoutParams, updateDbStreamed, updateLayoutStreamed} from "./tasks";
import {WorkerTaskType} from "./tasks/types";
import {itemsDb, type SvItem} from "$lib/stores";
import type {Transaction} from "dexie";
import {getLogger} from "$lib/logger";

const logger = getLogger()

const activeTasks = new Map<WorkerTaskType, { controller: AbortController, promise: Promise<any> }>();

self.onmessage = async function (msg: WorkerMessage<WorkerMessageData<any>>) {
    logger.debug('Got message', msg.data);
    const {task, aborted, payload} = msg.data;

    if (aborted) {
        cancelTask(task);
        return;
    }

    if (activeTasks.has(task)) {
        tryToUpdateTask(task, payload)
    } else {
        startTask(task, payload);
    }
};

function hookCreate(key: string, item: SvItem, transaction: Transaction) {
    logger.debug('Hook create', key, item)
    postMessage({task: WorkerTaskType.UpdateDb, status: "add_item", payload: item})
}

function hookUpdate(mods: Object, key: string, item: SvItem, transaction: Transaction) {
    logger.debug('Hook update', key, item)

    if (mods.hasOwnProperty('width') || mods.hasOwnProperty('height')) {
        postMessage({task: WorkerTaskType.UpdateDb, status: "update_item", payload: item})
    }
}

function restartLayoutFromPosition(item: SvItem) {
    layoutParams.restart = true;
    findItemIndex(item).then(index => {
        if (index < layoutParams.current) {
            layoutParams.current = index;
        }
    });
}

async function findItemIndex(item: SvItem): Promise<number> {
    let index = 0;
    let found = false;

    // todo find all with the same `row` field and return first of them
    // Means: "first from the same row"
    await itemsDb.items.orderBy('guid').each((dbItem: SvItem) => {
        if (dbItem.guid === item.guid) {
            found = true;
            return;
        }
        index++;
    });

    return found ? index : -1;
}

function startTask(task: WorkerTaskType, payload: any) {
    logger.info('Start task', WorkerTaskType[task], payload)
    const controller = new AbortController();

    const taskPromise = (async () => {
        try {
            switch (task) {
                case WorkerTaskType.UpdateDb:
                    await startUpdateDbTask(controller, payload)
                    break;

                case WorkerTaskType.UpdateLayout:
                    await startUpdateLayoutTask(controller, payload)
                    break;
            }
            postMessage({task, status: "completed"});
        } catch (error) {
            postMessage({task, status: "error"});
            logger.error(`Error in worker task ${task}:`, error);
        } finally {
            activeTasks.delete(task);
            postMessage(Idle);
            logger.info(`Task finalized`, WorkerTaskType[task]);
        }
    })();

    activeTasks.set(task, {controller, promise: taskPromise});
}

async function startUpdateDbTask(controller: AbortController, payload: UpdateDbPayload) {
    itemsDb.items.hook.creating.subscribe(hookCreate);
    itemsDb.items.hook.updating.subscribe(hookUpdate);
    try {
        await updateDbStreamed(controller.signal, payload);
    } finally {
        itemsDb.items.hook.creating.unsubscribe(hookCreate);
        itemsDb.items.hook.updating.unsubscribe(hookUpdate);
    }
}

async function startUpdateLayoutTask(controller: AbortController, payload: UpdateLayoutPayload) {
    await updateLayoutStreamed(controller.signal, payload);
}

function tryToUpdateTask(task: WorkerTaskType, payload: any) {
    logger.info('Update task', WorkerTaskType[task], payload)

    switch (task) {
        case WorkerTaskType.UpdateLayout:
            if (payload.item) {
                restartLayoutFromPosition(payload.item);
            } // todo else should i do something or just pass?
            break;

        default:
            postMessage({task, status: "cant_update"});
            break;
    }
    return;
}

function cancelTask(task: WorkerTaskType) {
    if (!activeTasks.has(task)) {
        postMessage({task, status: "not_running"});
        return;
    }

    const {controller} = activeTasks.get(task)!;
    controller.abort();
    activeTasks.delete(task);

    postMessage({task, status: "cancelled"});
    postMessage(Idle);
}
