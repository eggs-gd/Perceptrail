<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import ViewerTools from "$lib/gallery/components/ViewerTools.svelte";
    import {page} from '$app/state';
    import {layoutDb} from "$lib/stores";
    // Not re-exported from $lib/stores: the workers import that, this is page-only (svelte/reactivity)
    import {LiveQuery} from "$lib/stores/internal/liveQuery";
    import type {Attachment} from "svelte/attachments";

    const MIN_ZOOM = 0.25;
    const MAX_ZOOM = 8;

    let index = $derived(Number(page.params.index));

    // The gallery only holds the visible window, so the item comes from layoutDb.
    // /3 → /4 reuses this component, and on a direct /N load the item is written by
    // the layout worker after the first render — hence a live query per index.
    // An absent /N gives undefined, so the previous photo is not kept.
    let itemQuery = $derived.by(() => {
        const order = index;
        return new LiveQuery(() => layoutDb.items.where('order').equals(order).first());
    });
    let item = $derived(itemQuery.current);

    // The Original switch is per item: the next one opens with its preview again
    let originalFor = $state<string | null>(null);
    let showOriginal = $derived(!!item && originalFor === item.guid);

    // Zoom is remembered per item, so switching items starts at 1× again
    let zoomState = $state({guid: '', value: 1});
    let zoom = $derived(zoomState.guid === item?.guid ? zoomState.value : 1);

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

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="viewer"
     {@attach wheelZoom}
     onclick={() => history.back()}>
    {#if item}
        <div class="stage" style:transform="scale({zoom})">
            <ItemView {item} {index} mode="view" sizes="100vw" {showOriginal}/>
        </div>
    {/if}
</div>
<!-- Outside the zoomed stage: a transform would make the toolbar scale and move -->
{#if item?.asset}
    <ViewerTools asset={item.asset} {showOriginal}
                 ontoggleoriginal={() => (originalFor = showOriginal ? null : item!.guid)}/>
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
