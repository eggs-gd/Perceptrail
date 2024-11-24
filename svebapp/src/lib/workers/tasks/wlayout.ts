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
let msgPrt: MessagePort;
let currentRowHeight: number;
let currentScreenWidth: number;

self.onmessage = async function (msg: { data: WorkerMessage<any, any> }) {
    const {task, payload} = msg.data;

    const p: UpdateLayoutPayload = payload;

    if (task === 'init') {
        msgPrt = payload;
        msgPrt.onmessage = restartLayoutFromPosition;
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
            try {
                await updateLayoutStreamed(controller.signal, payload);
                postMessage({status: "completed"});
            } catch (error) {
                postMessage({status: "error"});
                logger.error("Error in UpdateLayout task:", error);
            } finally {
                currentTask = null;
            }
        })(),
    };
}

function restartLayoutFromPosition(event: { data: MessageFromSync }) {
    params.restart = true;
    findItemIndex(event.data.item).then(index => {
        if (index < params.current) {
            params.current = index;
        }
    });
}

async function findItemIndex(item: Item): Promise<number> {
    let index = 0;
    let found = false;

    // todo find all with the same `row` field and return first of them
    // Means: "first from the same row"
    await itemsDb.items.orderBy('guid').each((dbItem: Item) => {
        if (dbItem.guid === item.guid) {
            found = true;
            return;
        }
        index++;
    });

    return found ? index : -1;
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
    let rowLength = 0;
    const scale = payload.rowHeight / item.height;
    let lItem: LayoutItem = {
        ...item,
        order,
        scale,
        row: params.currentRowNum
    }

    params.currentRow.forEach((itm) => {
        rowLength += itm.width * itm.scale;
    })

    const approxWidth = rowLength + lItem.width * lItem.scale;
    const deltaWidth = payload.screenWidth - approxWidth;

    if (deltaWidth >= 0) {
        // can add to current row and proceed
        params.currentRow.push(lItem);
        return [];
    } else if (-deltaWidth < (lItem.width * lItem.scale) * 0.5) {
        // new item not match the total width more than half it's size
        // scale up current row. Start new row with this one item
        const res = params.currentRow;
        const finalScale = payload.screenWidth / approxWidth;
        res.forEach(itm => {
            itm.scale *= finalScale;
        })

        params.currentRowNum++;
        lItem.row = params.currentRowNum;
        params.currentRow = [lItem];
        return res;
    } else {
        // new item not match the total with less than half it's size
        // add to row and scale down whole row
        params.currentRow.push(lItem);
        const res = params.currentRow;
        const finalScale = payload.screenWidth / approxWidth;
        res.forEach(itm => {
            itm.scale *= finalScale;
        })

        params.currentRowNum++;
        params.currentRow = [];
        return res;
    }
}
