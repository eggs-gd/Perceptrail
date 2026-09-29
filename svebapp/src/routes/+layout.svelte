<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {currentIndex, currentItem, type LayoutItem} from "$lib/stores";
    import {goto} from "$app/navigation";
    import {page} from "$app/stores";

    let {children} = $props();

    let viewingItem = $derived($page.params.index != null);

    $effect(() => {
        document.body.style.overflow = viewingItem ? 'hidden' : '';
        return () => {
            document.body.style.overflow = '';
        };
    });

    function selectItem(item: LayoutItem) {
        $currentIndex = item.order;
        $currentItem = item;
    }

    function openItem(item: LayoutItem) {
        goto("/" + item.order, {noScroll: true});
    }
</script>

<main class:viewing={viewingItem}>
    <!-- Stays mounted under / and /[index] so gallery doesn't remount -->
    <Gallery selectItem={selectItem} openItem={openItem}/>
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
