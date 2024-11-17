import {type UpdateLayoutPayload} from "../types";
import {itemsDb, layoutDb, type LayoutItem, type SvItem} from "../idb";
import {getLogger} from "$lib/logger/logger";
import type {WorkerTask} from "./types";

const logger = getLogger()

interface Parameters {
    currentRowNum: number,
    currentRow: LayoutItem[],
    current: number,
    restart: boolean,
}

export const params: Parameters = {
    currentRowNum: 0,
    currentRow: [],
    current: 0,
    restart: false,
}


class RestartError extends Error {
    constructor(message = "The query should be restarted") {
        super(message);
        this.name = "RestartError";
    }
}

function throwIfNeedRestart(): void {
    if (params.restart) {
        throw new RestartError();
    }
}

export function isRestartError(error: unknown): error is RestartError {
    return error instanceof RestartError;
}

export const updateLayoutStreamed: WorkerTask<UpdateLayoutPayload> = async (signal: AbortSignal, payload: UpdateLayoutPayload) => {
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
            throwIfNeedRestart()

            const nextRow = placeInLayout(item, payload, params.current);

            signal.throwIfAborted()
            throwIfNeedRestart()

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


function placeInLayout(item: SvItem, payload: UpdateLayoutPayload, order: number): LayoutItem[] {
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
