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

export interface WorkerMessage<T> extends ExtendableMessageEvent {
    data: WorkerMessageData<T>
}

// Concrete messages

export interface UpdatePayload {
    api:string,
    fetch?: {
        (input: (RequestInfo | URL), init?: RequestInit): Promise<Response>
        (input: (string | URL | globalThis.Request), init?: RequestInit): Promise<Response>
    }
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
