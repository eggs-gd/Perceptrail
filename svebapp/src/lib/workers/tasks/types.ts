import type {Item, LayoutItem} from "$lib/stores";

export type WorkerTask<TData, TResult = void> = (
    signal: AbortSignal,
    data: TData
) => Promise<TResult>;

export class AbortError extends Error {
    constructor(message = "The operation was aborted") {
        super(message);
        this.name = "AbortError";
    }
}

export class RestartError extends Error {
    constructor(message = "The query should be restarted") {
        super(message);
        this.name = "RestartError";
    }
}

export function isRestartError(error: Error): error is RestartError {
    return error instanceof RestartError;
}

export function throwIfNeedRestart(restart: boolean): void {
    if (restart) {
        throw new RestartError();
    }
}

export function isAbortError(error: Error): error is AbortError {
    return error instanceof AbortError;
}

export type WorkerTaskType = 'init' | 'start' | 'update' | 'order' | 'mode';

export interface WorkerMessage<T, T1> {
    task: T;
    payload: T1
}

export interface UpdateLayoutPayload {
    screenWidth: number;
    rowHeight: number;
    /** guid of the first visible item: the relayout reports where it moved */
    anchor?: string;
}

/** The sheet's order from a perceptor (GET /p/:name/order), kept around the anchor */
export interface OrderPayload {
    /** Echoed in the OrderResult */
    id: number;
    url: string;
    /** guid to keep in view: the relayout reports where it moved */
    anchor?: string;
}

/** One line of /p/:name/order */
export interface OrderEntry {
    guid: string;
    /** The sections this item starts, coarsest first (a path, or one tag) */
    sections?: {level: number, label: string}[];
}

export type InitMessage = WorkerMessage<'init', MessagePort[]>;
export type OrderMessage = WorkerMessage<'order', OrderPayload>;

/** wlayout → page: whether an order was applied (a later one supersedes it: false) */
export interface OrderResult {
    task: 'order';
    id: number;
    ok: boolean;
}
export type UpdateLayoutMessage = WorkerMessage<'update', UpdateLayoutPayload>
export type StartSyncMessage = WorkerMessage<'start', string>

export interface WorkerEventData {
    status: string,
}

/** wsync → wlayout. The layout itself reaches the page through layoutDb (liveQuery). */
export interface MessageFromSync {
    item?: Item | LayoutItem,
    action: 'create' | 'update' | 'delete' | 'sync-start' | 'sync-done',
}

export type CurrentWorkerTask = { controller: AbortController; promise: Promise<void> } | null;
