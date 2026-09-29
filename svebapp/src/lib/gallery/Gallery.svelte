<script lang="ts">
    import {fade} from 'svelte/transition';
    import ItemView from "./components/ItemView.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {screenWidth} from "$lib/stores";


    interface Props {
        images: LayoutItem[] | undefined;
        gutter?: number;
        selectItem: (item: number) => void,
        openItem: (item: number) => void,
    }

    let {
        images = [],
        gutter = 8,
        selectItem,
        openItem
    }: Props = $props();

    // Items are ordered, so the last one sits in the lowest row
    let height = $derived.by(() => {
        const last = images[images.length - 1];
        return last ? last.y + last.h + gutter : 0;
    });

    // Wave on relayout: tiles that stay in their row only rescale right away;
    // tiles that move to another row start one after another (visible ones only).
    // The cap keeps a window-edge drag responsive: a new layout arrives every ~33ms
    // and restarts pending delays, so they must stay short.
    const STAGGER_STEP_MS = 60;
    const STAGGER_MAX_MS = 500;

    let containerEl: HTMLDivElement | undefined = $state();

    // Row of each tile in the previous layout (plain variable: bookkeeping, not state)
    let prevRows = new Map<string, number>();

    let delays = $derived.by(() => {
        const result = new Map<string, number>();
        // One layout read per new layout (not per tile, not per scroll)
        const top = containerEl?.getBoundingClientRect().top ?? 0;
        const viewportHeight = typeof window === 'undefined' ? 0 : window.innerHeight;

        let k = 0;
        for (const itm of images) {
            const prevRow = prevRows.get(itm.guid);
            const visible = top + itm.y + itm.h > 0 && top + itm.y < viewportHeight;
            if (prevRow !== undefined && prevRow !== itm.row && visible) {
                result.set(itm.guid, Math.min(k * STAGGER_STEP_MS, STAGGER_MAX_MS));
                k++;
            }
        }

        prevRows = new Map(images.map((itm) => [itm.guid, itm.row]));
        return result;
    });

    // Positions come from the layout worker. Tiles are absolutely positioned and
    // keyed by guid, so on relayout (resize) the same DOM node moves to its new
    // place — also into another row — through the CSS transition below.
    function tileStyle(itm: LayoutItem, delay: number) {
        return `transform: translate(${itm.x}px, ${itm.y}px); width: ${itm.w}px; height: ${itm.h}px;`
            + (delay ? ` transition-delay: ${delay}ms;` : '');
    }

</script>

<div class="masonry" bind:clientWidth={$screenWidth}>
    <div class="container" bind:this={containerEl} style="height: {height}px" class:hidden={!$screenWidth}>
        {#each images as itm (itm.guid)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="image"
                 in:fade={{ duration: 300 }}
                 style={tileStyle(itm, delays.get(itm.guid) ?? 0)}
                 onclick={() => {
                     selectItem(itm.order);
                     openItem(itm.order);
                 }}>
                <ItemView item={itm} index={itm.order}/>
            </div>
        {/each}
    </div>
</div>

<style>
    .masonry {
        max-width: 100%;
    }

    .container {
        position: relative;
    }

    .image {
        position: absolute;
        top: 0;
        left: 0;
        box-sizing: border-box;
        border: 1px solid green;
        transition:
            transform 500ms ease,
            width 200ms ease,
            height 200ms ease;
    }

    @media (prefers-reduced-motion: reduce) {
        .image {
            transition: none;
        }
    }

    .image > :global(*) {
        width: 100%;
        height: 100%;
    }

    .hidden {
        visibility: hidden;
    }

</style>
