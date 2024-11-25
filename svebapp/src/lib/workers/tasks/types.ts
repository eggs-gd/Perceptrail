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

export type WorkerTaskType = 'init' | 'start' | 'update';

export interface WorkerMessage<T, T1> {
    task: T;
    payload: T1
}

export interface UpdateLayoutPayload {
    screenWidth: number;
    rowHeight: number;
}

export type InitMessage = WorkerMessage<'init', MessagePort[]>;
export type UpdateLayoutMessage = WorkerMessage<'update', UpdateLayoutPayload>
export type StartSyncMessage = WorkerMessage<'start', string>

export interface WorkerEventData {
    status: string,
}

export interface MessageFromSync {
    item: Item | LayoutItem,
    action: 'create' | 'update' | 'delete',
}

export type CurrentWorkerTask = { controller: AbortController; promise: Promise<void> } | null;
