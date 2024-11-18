import type {Readable} from "svelte/store";
import {liveQuery} from "dexie";

export function dexieStore<T>(querier: () => T | Promise<T>): Readable<T> {
    const dexieObservable = liveQuery(querier)
    return {
        subscribe(run, invalidate) {
            return dexieObservable.subscribe(run, invalidate).unsubscribe

        }
    }
}
