/**
 * List of available worker tasks
 */
export enum WorkerTask {
    UpdateDb,
}

// Abstract worker message (requests)

export interface WorkerMessageData<T> {
    task: WorkerTask;
    payload?: T
}

export interface WorkerMessage<T> extends MessageEvent {
    data: WorkerMessageData<T>
}

// Concrete messages

export interface UpdatePayload {
    api:string,
}

export interface UpdateMessageData extends WorkerMessageData<UpdatePayload> {
    task: WorkerTask.UpdateDb,
}

// Concrete events

export interface IdleEventData {
    idle: boolean
}

export const Idle: IdleEventData = {
    idle: true
}
