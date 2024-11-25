import {
    type CurrentWorkerTask,
    type MessageFromSync,
    RestartError,
    throwIfNeedRestart,
    type UpdateLayoutPayload,
    type WorkerMessage,
    type WorkerTask
} from "./types";
import {type Item, itemsDb, layoutDb, type LayoutItem} from "$lib/stores";
import {getLogger} from "$lib/logger";

const logger = getLogger()

interface Parameters {
    currentRowNum: number,
    currentRow: LayoutItem[],
    current: number,
    restart: boolean,
}

const params: Parameters = {
    currentRowNum: 0,
    currentRow: [],
    current: 0,
    restart: false,
}

let currentTask: CurrentWorkerTask = null;
let itemsDbPort: MessagePort;
let updatesPort: MessagePort;
let currentRowHeight: number;
let currentScreenWidth: number;

self.onmessage = async function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    const p: UpdateLayoutPayload = payload;

    if (task === 'init') {
        itemsDbPort = payload[0];
        itemsDbPort.onmessage = restartLayoutFromPosition;
        updatesPort = payload[1];
        logger.debug('Inited')
    } else if (task === 'start') {
        logger.debug('Trying to start')
    } else if (task === 'update' && !currentTask) {
        currentScreenWidth = p.screenWidth;
        currentRowHeight = p.rowHeight;
        currentTask = startNewTask(payload)
        logger.debug('Started')
    } else if (task === 'update' && currentTask) {
        if (currentScreenWidth === p.screenWidth
            && currentRowHeight === p.rowHeight) {
            postMessage({task, status: "cant_update"});
        } else {
            currentTask.controller.abort()
            currentTask = startNewTask(payload)
            logger.debug('Restarted')
        }
    }
};

function startNewTask(payload: UpdateLayoutPayload) {
    const controller = new AbortController();
    return {
        controller,
        promise: (async () => {
            layoutDb.items.hook.creating.subscribe(hookCreate);
            layoutDb.items.hook.updating.subscribe(hookUpdate);
            layoutDb.items.hook.deleting.subscribe(hookDelete);
            try {
                await updateLayoutStreamed(controller.signal, payload);
                postMessage({status: "completed"});
            } catch (error) {
                postMessage({status: "error"});
                logger.error("Error in UpdateLayout task:", error);
            } finally {
                layoutDb.items.hook.creating.unsubscribe(hookCreate);
                layoutDb.items.hook.updating.unsubscribe(hookUpdate);
                layoutDb.items.hook.deleting.unsubscribe(hookDelete);
                currentTask = null;
            }
        })(),
    };
}

function hookCreate(key: string, item: LayoutItem) {
    const msg: MessageFromSync = {action: "create", item: item};
    updatesPort.postMessage(msg);
}

function hookUpdate(mods: Object, key: string, item: LayoutItem) {
    const msg: MessageFromSync = {action: "update", item: item};
    updatesPort.postMessage(msg);
}

function hookDelete(key: string, item: Item) {
    const msg: MessageFromSync = {action: "delete", item: item};
    updatesPort.postMessage(msg);
}

function restartLayoutFromPosition(event: MessageEvent<MessageFromSync>) {
    if (!currentTask) {
        currentTask = startNewTask({screenWidth: currentScreenWidth, rowHeight: currentRowHeight})
        return;
    }

    findItemIndex(event.data.item).then(item => {
        if (!item) {
            params.current = 0; //???
            params.currentRowNum = 0;
        } else if (item.order < params.current) {
            params.current = item.order;
            params.currentRowNum = item.row;
        }

        params.restart = true;
        params.currentRow = [];
    });
}

async function findItemIndex(item: Item): Promise<LayoutItem | undefined> {
    let index = 0;
    let found = false;

    await itemsDb.items.orderBy('guid').each((dbItem: Item) => {
        if (dbItem.guid === item.guid) {
            found = true;
            return;
        }
        index++;
    });

    if (!found) return undefined;

    const layoutItem = await layoutDb.items
        .where('order')
        .equals(index - 1)
        .first();

    if (!layoutItem) return undefined;

    const row = layoutItem.row;

    const smallestOrderItem = await layoutDb.items
        .where('row')
        .equals(row)
        .sortBy('order');

    if (!smallestOrderItem.length) return undefined;

    return smallestOrderItem[0];
}

const updateLayoutStreamed: WorkerTask<UpdateLayoutPayload> = async (signal: AbortSignal, payload: UpdateLayoutPayload) => {
    signal.throwIfAborted();

    params.current = 0;
    params.currentRowNum = 0;
    params.currentRow = [];
    params.restart = false

    await runMagic(signal, payload);
}

async function runMagic(signal: AbortSignal, payload: UpdateLayoutPayload) {
    logger.info('runMagic');
    do {
        params.restart = false;
        const cursor = itemsDb.items.orderBy('guid').offset(params.current);

        await cursor.each((item) => {
            signal.throwIfAborted()
            throwIfNeedRestart(params.restart)

            const nextRow = placeInLayout(item, payload, params.current);

            signal.throwIfAborted()
            throwIfNeedRestart(params.restart)

            if (nextRow.length > 0) {
                logger.info('add next row', params.currentRowNum, nextRow)
                layoutDb.items.bulkPut(nextRow)
            }

            ++params.current;
        }).catch(RestartError, (error) => {
            logger.info('restarting layout from index', params.current, error)
        }).catch(error => {
            logger.error('got error', error)
            throw error
        });
    } while (params.restart)
}


function placeInLayout(item: Item, payload: UpdateLayoutPayload, order: number): LayoutItem[] {
    let rowLength = params.currentRow.reduce((sum, itm) => sum + itm.width * itm.scale, 0);
    const scale = payload.rowHeight / item.height;
    let lItem: LayoutItem = {
        ...item,
        order,
        scale,
        row: params.currentRowNum,
    };

    const approxWidth = rowLength + lItem.width * lItem.scale;
    const deltaWidth = payload.screenWidth - approxWidth;

    if (deltaWidth >= 0) {
        // Can fit in the current row
        params.currentRow.push(lItem);
        return [];
    }

    const newItemWidth = lItem.width * lItem.scale;
    if (-deltaWidth < newItemWidth * 0.5) {
        // Scale up current row
        const res = params.currentRow;
        const finalScale = payload.screenWidth / (rowLength + newItemWidth); // Only include actual row length
        res.forEach(itm => {
            itm.scale *= finalScale;
        });

        params.currentRowNum++;
        lItem.row = params.currentRowNum;
        params.currentRow = [lItem];
        return res;
    }

    // Add to row and scale down the entire row
    params.currentRow.push(lItem);
    const res = params.currentRow;
    const finalScale = payload.screenWidth / approxWidth; // Adjust for the final width
    res.forEach(itm => {
        itm.scale *= finalScale;
    });

    params.currentRowNum++;
    params.currentRow = [];
    return res;
}

//
// function placeInLayout(item: Item, payload: UpdateLayoutPayload, order: number): LayoutItem[] {
//     let rowLength = 0;
//     const scale = payload.rowHeight / item.height;
//     let lItem: LayoutItem = {
//         ...item,
//         order,
//         scale,
//         row: params.currentRowNum
//     }
//
//     params.currentRow.forEach((itm) => {
//         rowLength += itm.width * itm.scale;
//     })
//
//     const approxWidth = rowLength + lItem.width * lItem.scale;
//     const deltaWidth = payload.screenWidth - approxWidth;
//
//     if (deltaWidth >= 0) {
//         // can add to current row and proceed
//         params.currentRow.push(lItem);
//         return [];
//     } else if (-deltaWidth < (lItem.width * lItem.scale) * 0.5) {
//         // new item not match the total width more than half it's size
//         // scale up current row. Start new row with this one item
//         const res = params.currentRow;
//         const finalScale = payload.screenWidth / approxWidth;
//         res.forEach(itm => {
//             itm.scale *= finalScale;
//         })
//
//         params.currentRowNum++;
//         lItem.row = params.currentRowNum;
//         params.currentRow = [lItem];
//         return res;
//     } else {
//         // new item not match the total with less than half it's size
//         // add to row and scale down whole row
//         params.currentRow.push(lItem);
//         const res = params.currentRow;
//         const finalScale = payload.screenWidth / approxWidth;
//         res.forEach(itm => {
//             itm.scale *= finalScale;
//         })
//
//         params.currentRowNum++;
//         params.currentRow = [];
//         return res;
//     }
// }
