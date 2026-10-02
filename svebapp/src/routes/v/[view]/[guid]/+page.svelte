<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import ViewerTools from "$lib/gallery/components/ViewerTools.svelte";
    import InfoPanel from "$lib/gallery/components/InfoPanel.svelte";
    import {toggleViewerPref, viewerPrefs} from "$lib/gallery/components/viewerPrefs.svelte";
    import {photoHref, viewHref} from "$lib/gallery/perceptors.svelte";
    import {page} from '$app/state';
    import {goto} from '$app/navigation';
    import {layoutDb} from "$lib/stores";
    // Not re-exported from $lib/stores: the workers import that, this is page-only (svelte/reactivity)
    import {LiveQuery} from "$lib/stores/internal/liveQuery";
    import type {Attachment} from "svelte/attachments";

    const MIN_ZOOM = 0.25;
    const MAX_ZOOM = 8;

    // /v/<view>/<guid>: the photo by its guid (a link stays the same photo whatever
    // the view or new photos do); ← → walk the view's sheet
    let view = $derived(page.params.view!);
    let guid = $derived(page.params.guid!);

    // The gallery only holds the visible window, so the item comes from layoutDb. On a
    // direct load it is written by the layout worker after the first render — hence a
    // live query per guid; an unknown guid gives undefined.
    let itemQuery = $derived.by(() => {
        const g = guid;
        return new LiveQuery(() => layoutDb.items.get(g));
    });
    let item = $derived(itemQuery.current);
    let index = $derived(item?.order ?? 0);

    // The info panel: its button is a switch kept like the others (photo to photo,
    // between visits)
    const toggleInfo = () => toggleViewerPref('infoOpen');

    // The Original switch is per item: the next one opens with its preview again
    let originalFor = $state<string | null>(null);
    let showOriginal = $derived(!!item && originalFor === item.guid);

    // Zoom is remembered per item, so switching items starts at 1× again
    let zoomState = $state({guid: '', value: 1});
    let zoom = $derived(zoomState.guid === item?.guid ? zoomState.value : 1);

    // Close: back to the gallery entry it was opened from; opened directly (a link, a
    // reload of a replaced entry) there is none — go to the sheet around this photo
    // instead of leaving the site
    function close() {
        if (page.state.fromGallery) history.back();
        else goto(viewHref(view, guid), {replaceState: true, noScroll: true});
    }

    // ← →: the previous / next item of the sheet. The entry is replaced, so Back (and
    // close) still leads to the gallery, not through every item seen. Steps count from
    // the item being navigated to (a held key or quick presses add up); only the latest
    // step navigates, past the end it stays.
    let pending: number | undefined;
    function step(delta: number) {
        if (!item) return;
        const next = (pending ?? item.order) + delta;
        if (next < 0) return;
        pending = next;
        layoutDb.items.where('order').equals(next).first().then((found) => {
            if (pending !== next) return; // a later step took over
            if (!found) {
                pending = undefined;
                return;
            }
            goto(photoHref(view, found.guid), {replaceState: true, noScroll: true, keepFocus: true, state: page.state})
                .finally(() => {
                    if (pending === next) pending = undefined;
                });
        });
    }

    // A view picked here: the sheet in it around this photo (the viewer's entry is
    // replaced — Back goes to where the viewer was opened from)
    function pickView(slug: string) {
        goto(viewHref(slug, guid), {replaceState: true, noScroll: true});
    }

    function onkeydown(e: KeyboardEvent) {
        if (e.metaKey || e.ctrlKey || e.altKey) return;
        if (e.key === 'Escape') close();
        else if (e.key === 'ArrowRight') step(1);
        else if (e.key === 'ArrowLeft') step(-1);
        else return;
        e.preventDefault();
    }

    // onwheel={...} would be passive: preventDefault() needs a manual listener
    const wheelZoom: Attachment<HTMLElement> = (node) => {
        const onWheel = (e: WheelEvent) => {
            e.preventDefault();
            if (!item) return;
            const factor = e.deltaY > 0 ? 0.9 : 1.1;
            zoomState = {guid: item.guid, value: Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom * factor))};
        };
        node.addEventListener('wheel', onWheel, {passive: false});
        return () => node.removeEventListener('wheel', onWheel);
    };
</script>

<svelte:window {onkeydown}/>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="viewer"
     {@attach wheelZoom}
     onclick={close}>
    {#if item}
        <div class="stage" style:transform="scale({zoom})">
            <ItemView {item} {index} mode="view" sizes="100vw" {showOriginal}/>
        </div>
    {/if}
</div>
<!-- Outside the zoomed stage: a transform would make the toolbar scale and move -->
{#if item?.asset}
    <ViewerTools asset={item.asset} {showOriginal}
                 ontoggleoriginal={() => (originalFor = showOriginal ? null : item!.guid)}
                 onperceptor={pickView}
                 infoOpen={viewerPrefs.infoOpen} ontoggleinfo={toggleInfo}/>
{/if}
{#if viewerPrefs.infoOpen}
    <InfoPanel {guid} asset={item?.asset} onclose={toggleInfo}/>
{/if}

<style>
    .viewer {
        position: fixed;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        overflow: hidden;
        background: var(--color-bg);
    }

    .stage {
        max-width: 100%;
        max-height: 100%;
        transform-origin: center center;
        line-height: 0;
    }

    .stage :global(img),
    .stage :global(video) {
        display: block;
        max-width: 100vw;
        max-height: 100vh;
        width: auto;
        height: auto;
        object-fit: contain;
    }
</style>
