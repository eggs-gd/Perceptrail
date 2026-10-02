<script lang="ts">
    import "../app.css";
    import Gallery from "$lib/gallery/Gallery.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {loadFromServer, refreshFromServer, setWorkersMode} from "$lib/workers";
    import {afterNavigate} from "$app/navigation";
    import {loadApp} from "$lib/app.svelte";
    import {loadPerceptors, photoHref} from "$lib/gallery/perceptors.svelte";
    import {goto} from "$app/navigation";
    import {page} from "$app/state";
    import {onMount} from "svelte";

    let {children} = $props();

    // /v/<view> — the sheet in a view; /v/<view>/<guid> — the viewer on a photo
    let view = $derived(page.params.view);
    let viewing = $derived(page.params.guid);
    let viewingItem = $derived(viewing != null);

    // Start the sync once, in the browser. Not in a load function: load must stay free
    // of side effects (it also runs on the server and on every navigation).
    const REFRESH_EVERY_MS = 20_000;

    onMount(() => {
        loadApp().then(setWorkersMode);
        loadFromServer();
        // The views; the URL says which one the sheet is in (the stream itself comes
        // newest first, so the photos show before the order arrives)
        loadPerceptors();
        // What changed on the server: on coming back to the tab, on every navigation,
        // and every 20 s while the tab is seen (no push from the server yet: an
        // original Photos downloaded, a new import show up by themselves; an empty
        // delta is 0 bytes)
        const onVisible = () => {
            if (document.visibilityState === 'visible') refreshFromServer();
        };
        document.addEventListener('visibilitychange', onVisible);
        const timer = setInterval(onVisible, REFRESH_EVERY_MS);
        return () => {
            document.removeEventListener('visibilitychange', onVisible);
            clearInterval(timer);
        };
    });
    afterNavigate(() => refreshFromServer());

    $effect(() => {
        document.body.style.overflow = viewingItem ? 'hidden' : '';
        return () => {
            document.body.style.overflow = '';
        };
    });

    function openItem(item: LayoutItem) {
        if (!view) return;
        goto(photoHref(view, item.guid), {noScroll: true, state: {fromGallery: true}});
    }
</script>

<main class={{viewing: viewingItem}}>
    <!-- Stays mounted under /v/<view> and /v/<view>/<guid> so the gallery doesn't remount -->
    <Gallery {openItem} {view} {viewing}/>
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
