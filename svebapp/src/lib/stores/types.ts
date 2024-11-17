import type {WorkerTaskType} from "./tasks/types";
import type {SvItem} from "$lib/stores/idb";

export class AbortError extends Error {
    constructor(message = "The operation was aborted") {
        super(message);
        this.name = "AbortError";
    }
}

export function isAbortError(error: unknown): error is AbortError {
    return error instanceof AbortError;
}

// Abstract worker message (requests)

export interface WorkerMessageData<T, TaskType = WorkerTaskType> {
    aborted: boolean;
    task: TaskType;
    payload?: T;
}

export type WorkerMessage<T, TaskType = WorkerTaskType> = MessageEvent & {
    data: WorkerMessageData<T, TaskType>;
};

// Concrete messages

export interface UpdateDbPayload {
    api: string;
}

export type UpdateDbMessageData = WorkerMessageData<UpdateDbPayload, WorkerTaskType.UpdateDb>;

export interface UpdateLayoutPayload {
    screenWidth: number;
    rowHeight: number;
    item?:SvItem,
    added?:boolean,
}

export type UpdateLayoutMessageData = WorkerMessageData<UpdateLayoutPayload, WorkerTaskType.UpdateLayout>;

// Concrete events

export interface WorkerEventData {
    task:WorkerTaskType,
    status:string,
    payload?:any,
}

export interface IdleEventData {
    idle: boolean
}

export const Idle: IdleEventData = {
    idle: true
}
