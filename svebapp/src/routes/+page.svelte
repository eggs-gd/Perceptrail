<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {goto} from "$app/navigation";
    import {currentIndex, currentItem} from "./stores";
    import {db, dexieStore} from "./idb";
    import {liveQuery} from "dexie";

    //let items = dexieStore(async () => await db.items.toArray());
    let items = $derived(liveQuery(async () => await db.items.toArray()));

    function selectItem(index:number) {
        $currentIndex = index;
        $currentItem = $items[$currentIndex]
    }

    function openItem(index : number) {
        goto("/" + index,  { replaceState: true })
    }
</script>

<Gallery images={$items} rowHeight={200} selectItem={selectItem} openItem={openItem}/>
