<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {goto} from "$app/navigation";
    import {currentIndex, currentItem, items} from "./stores";

    function selectItem(e: CustomEvent<number>) {
        $currentIndex = e.detail;
        $currentItem = $items[$currentIndex]
    }

    function openItem(e: CustomEvent<number>) {
        goto("/" + e.detail)
    }
</script>

<Gallery images={$items} rowHeight={950} on:selectItem={selectItem} on:openItem={openItem}/>
