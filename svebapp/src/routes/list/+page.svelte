<script lang="ts">
import {photos} from "$lib/data/photos";
import Gallery from "$lib/gallery";
import type {Item} from "$lib/gallery";


function prepareItems(input: {}[]): Item[]{
    const out:Item[] = []
    input.map((itm) => {
        out.push(itm as Item);
    })
    return out;
}

</script>

<Gallery images={prepareItems(photos)} rowHeight={950}/>
