import {throwIfAborted} from "../types";
import {itemsDb, layoutDb, type LayoutItem, type SvItem} from "../idb";
import type {Transaction} from "dexie";
import {Logger} from "$lib/logger";

const logger = new Logger()

let currentRowNum: number = 0;
let currentRow: LayoutItem[] = []
let current: number = 0;
let restart: boolean = false;

class RestartError extends Error {
    constructor(message = "The query should be restarted") {
        super(message);
        this.name = "RestartError";
    }
}

function throwIfNeedRestart(): void {
    if (restart) throw new RestartError();
}

export function isRestartError(error: unknown): error is RestartError {
    return error instanceof RestartError;
}

const updateLayout: WorkerTask<number> = async (signal: AbortSignal, width: number) => {
    throwIfAborted(signal);

    current = 0;

    itemsDb.items.hook.creating.subscribe(hookCreate)
    itemsDb.items.hook.updating.subscribe(hookUpdate)

    await runMagic(signal, width)
        .finally(() => {
            itemsDb.items.hook.creating.unsubscribe(hookCreate);
            itemsDb.items.hook.updating.unsubscribe(hookUpdate);
        });
}

async function runMagic(signal: AbortSignal, width: number) {
    do {
        restart = false;
        const cursor = itemsDb.items.orderBy('guid').offset(current);
        try {
            await cursor.each((item) => {
                throwIfAborted(signal)
                throwIfNeedRestart()

                const nextRow = placeInLayout(item, width, current);

                throwIfAborted(signal)
                throwIfNeedRestart()

                layoutDb.items.bulkPut(nextRow)
                ++current;
            })
        } catch (error) {
            if (isRestartError(error)) {
                logger.info('restarting layout from index', current)
            } else {
                throw error
            }
        }
    } while (restart)
}

async function hookCreate(key: string, item: SvItem, transaction: Transaction) {
    restart = true;
    const index = await findItemIndex(item);
    if (index < current) {
        current = index;
    }
}

async function hookUpdate(mods: Object, key: string, item: SvItem, transaction: Transaction) {
    if (mods.hasOwnProperty('width') || mods.hasOwnProperty('height')) {
        restart = true;
        const index = await findItemIndex(item);
        if (index < current) {
            current = index;
        }
    }
}

async function findItemIndex(item: SvItem): Promise<number> {
    let index = 0;
    let found = false;

    await itemsDb.items.orderBy('guid').each((dbItem) => {
        if (dbItem.guid === item.guid) {
            found = true;
            return;
        }
        index++;
    });

    return found ? index : -1;
}

function placeInLayout(item: SvItem, width: number, order: number): LayoutItem[] {
    const scale = 1;
    const row = 1;
    return [{
        // todo add real logic placing them to rows and make proper scales accordingly
        ...item,
        order,
        scale,
        row,
    }]
}

export default updateLayout
