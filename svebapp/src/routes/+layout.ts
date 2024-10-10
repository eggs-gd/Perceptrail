
import type {Item} from "$lib/gallery";
import {items} from "./stores";

function prepareItems(input: {}[]): Item[] {
    const out: Item[] = []
    input.map((itm) => {
        out.push(itm as Item);
    })
    return out;
}

export function load({ data }: {data:{items: {}[]}}) {
    items.set(prepareItems(data.items as {}[]));
    return {};
}
