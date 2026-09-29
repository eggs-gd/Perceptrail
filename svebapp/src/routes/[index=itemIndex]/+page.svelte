<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import {page} from '$app/stores';
    import {currentIndex, currentItem, items} from "$lib/stores";
    import {onMount} from "svelte";

    // Reactive, not a one-off at init: /3 → /4 reuses this component, and on a
    // direct /N load the items stream in after the first render
    $effect.pre(() => {
        const index = Number($page.params.index);
        $currentIndex = index;
        $currentItem = $items[index];
    });

    const MIN_ZOOM = 0.25;
    const MAX_ZOOM = 8;

    let zoom = $state(1);
    let viewerEl: HTMLDivElement | undefined = $state();

    $effect(() => {
        void $currentItem?.guid;
        zoom = 1;
    });

    function onWheel(e: WheelEvent) {
        e.preventDefault();
        const factor = e.deltaY > 0 ? 0.9 : 1.1;
        zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom * factor));
    }

    onMount(() => {
        const el = viewerEl;
        if (!el) return;
        el.addEventListener('wheel', onWheel, {passive: false});
        return () => el.removeEventListener('wheel', onWheel);
    });
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="viewer"
     bind:this={viewerEl}
     onclick={() => history.back()}>
    {#if $currentItem}
        <div class="stage" style="transform: scale({zoom})">
            <ItemView item={$currentItem} index={$currentIndex}/>
        </div>
    {/if}
</div>

<style>
    .viewer {
        position: fixed;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        overflow: hidden;
        background: #111;
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
