<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {updateLayout} from "$lib/workers";
    import {currentIndex, currentItem, items, rowHeight, screenWidth} from "$lib/stores";
    import {goto} from "$app/navigation";
    import {page} from "$app/stores";

    let {children} = $props();

    let viewingItem = $derived($page.params.index != null);

    $effect(() => updateLayout($screenWidth, $rowHeight));

    $effect(() => {
        document.body.style.overflow = viewingItem ? 'hidden' : '';
        return () => {
            document.body.style.overflow = '';
        };
    });

    function selectItem(index: number) {
        $currentIndex = index;
        $currentItem = $items[$currentIndex];
    }

    function openItem(index: number) {
        goto("/" + index, {noScroll: true});
    }
</script>

<main class:viewing={viewingItem}>
    <!-- Stays mounted under / and /[index] so gallery doesn't remount -->
    <Gallery images={$items} selectItem={selectItem} openItem={openItem}/>
    {@render children()}
</main>

<style>
    main.viewing {
        /* keep scroll position; viewer is fixed overlay */
        pointer-events: none;
    }

    main.viewing :global(.viewer) {
        pointer-events: auto;
    }
</style>
