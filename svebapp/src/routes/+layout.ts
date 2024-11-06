import {initializeStore} from "./stores";
import type {SvItem} from "$lib/server/server.api";


export async function load({data}: { data: { items: SvItem[] } }) {
    await initializeStore(data.items)
}
