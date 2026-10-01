<script lang="ts">
    import "../app.css";
    import Gallery from "$lib/gallery/Gallery.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {loadFromServer} from "$lib/workers";
    import {goto} from "$app/navigation";
    import {page} from "$app/state";
    import {onMount} from "svelte";

    let {children} = $props();

    let viewingItem = $derived(page.params.index != null);

    // Start the sync once, in the browser. Not in a load function: load must stay free
    // of side effects (it also runs on the server and on every navigation).
    onMount(loadFromServer);

    $effect(() => {
        document.body.style.overflow = viewingItem ? 'hidden' : '';
        return () => {
            document.body.style.overflow = '';
        };
    });

    function openItem(item: LayoutItem) {
        goto("/" + item.order, {noScroll: true, state: {fromGallery: true}});
    }
</script>

<main class={{viewing: viewingItem}}>
    <!-- Stays mounted under / and /[index] so gallery doesn't remount -->
    <Gallery {openItem} viewing={viewingItem ? Number(page.params.index) : undefined}/>
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
