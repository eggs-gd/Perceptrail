<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {goto} from "$app/navigation";
    import {currentIndex, currentItem} from "./stores";
    import {liveQuery} from "dexie";
    import {db} from "./idb";

    let items = $derived.by(() => liveQuery(
        () => db.items.toArray()
    ));

    function selectItem(index:number) {
        $currentIndex = index;
        $currentItem = $items[$currentIndex]
    }

    function openItem(index : number) {
        goto("/" + index,  { replaceState: true })
    }
</script>

<Gallery images={$items} rowHeight={200} selectItem={selectItem} openItem={openItem}/>
