import {liveQuery} from "dexie";
import {createSubscriber} from "svelte/reactivity";

/**
 * A Dexie liveQuery as a reactive value: `current` updates whenever the queried data
 * changes (also when a worker writes it). The subscription is only active while
 * `current` is read in an effect or template.
 *
 * Query parameters from state: create it in a $derived that reads them, so a new
 * query replaces the old one when they change:
 *   let query = $derived.by(() => { const order = index; return new LiveQuery(() => …order…); });
 */
export class LiveQuery<T> {
    #value: T | undefined;
    #subscribe: () => void;

    constructor(querier: () => T | Promise<T>) {
        this.#subscribe = createSubscriber((update) => {
            const sub = liveQuery(querier).subscribe({
                next: (value) => {
                    this.#value = value;
                    update();
                },
                error: (error) => console.error('liveQuery', error),
            });
            return () => sub.unsubscribe();
        });
    }

    get current(): T | undefined {
        this.#subscribe();
        return this.#value;
    }
}
