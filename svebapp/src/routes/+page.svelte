<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {goto} from "$app/navigation";
    import {currentIndex, currentItem, db} from "./stores";
    import {liveQuery} from "dexie";

    let items = liveQuery(
        () => db.items.toArray()
    );

    function selectItem(index:number) {
        $currentIndex = index;
        $currentItem = $items[$currentIndex]
    }

    function openItem(index : number) {
        goto("/" + index)
    }
</script>

<Gallery images={$items} rowHeight={950} selectItem={selectItem} openItem={openItem}/>
