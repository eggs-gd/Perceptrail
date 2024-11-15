export class AbortError extends Error {
    constructor(message = "The operation was aborted") {
        super(message);
        this.name = "AbortError";
    }
}

export function throwIfAborted(signal: AbortSignal): void {
    if (signal.aborted) throw new AbortError();
}

export function isAbortError(error: unknown): error is AbortError {
    return error instanceof AbortError;
}

// Abstract worker message (requests)

export interface WorkerMessageData<T, TaskType = WorkerTaskType> {
    task: TaskType;
    signal: AbortSignal;
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
    width: number;
}

export type UpdateLayoutMessageData = WorkerMessageData<UpdateLayoutPayload, WorkerTaskType.UpdateLayout>;

// Concrete events

export interface IdleEventData {
    idle: boolean
}

export const Idle: IdleEventData = {
    idle: true
}
