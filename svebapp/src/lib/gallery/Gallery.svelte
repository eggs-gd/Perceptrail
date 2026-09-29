<script lang="ts">
    import {fade} from 'svelte/transition';
    import {tick, untrack} from 'svelte';
    import ItemView from "./components/ItemView.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {rowHeight, screenWidth} from "$lib/stores";
    import {updateLayout} from "$lib/workers";
    import {type AnchorState, findAnchor, watchWindow, type WindowSnapshot} from "./layoutWindow";

    interface Props {
        gutter?: number;
        selectItem: (item: LayoutItem) => void,
        openItem: (item: LayoutItem) => void,
    }

    let {
        gutter = 8,
        selectItem,
        openItem
    }: Props = $props();

    let containerEl: HTMLDivElement | undefined = $state();
    let scrollY = $state(0);
    let innerHeight = $state(800);

    /** Visible items (plus overscan) from layoutDb, and the total height */
    let images: LayoutItem[] = $state([]);
    let height = $state(0);

    // Gallery top in document coordinates; the gallery is laid out from y = 0
    let containerTop = 0;
    const anchorState: AnchorState = {appliedRev: -1};

    // The window moves in steps of half a viewport: scrolling inside a step does not
    // re-create the subscription; the overscan (1 viewport above, 2 below) covers it.
    let step = $derived(Math.max(200, Math.round(innerHeight / 2)));
    let bucket = $derived(Math.floor(Math.max(0, scrollY - containerTop) / step));
    let range = $derived({top: (bucket - 2) * step, bottom: (bucket + 4) * step, viewport: innerHeight});

    // Width or row height changed: remember what is at the top of the viewport and
    // ask for a relayout that reports where that item ends up.
    $effect(() => {
        const width = $screenWidth;
        const targetRowHeight = $rowHeight;
        if (!width) return;
        untrack(() => {
            if (containerEl) containerTop = containerEl.getBoundingClientRect().top + window.scrollY;
            const anchor = findAnchor(images, window.scrollY - containerTop);
            anchorState.pending = anchor;
            updateLayout(width, targetRowHeight, anchor?.guid);
        });
    });

    $effect(() => {
        const sub = watchWindow(range, anchorState).subscribe({
            next: schedule,
            error: (error) => console.error('layout window', error),
        });
        return () => sub.unsubscribe();
    });

    // Apply at most one snapshot per frame (the stream re-runs the query per item)
    let latest: WindowSnapshot | undefined;
    let scheduled = false;

    function schedule(snapshot: WindowSnapshot) {
        latest = snapshot;
        if (scheduled) return;
        scheduled = true;
        // Hidden tabs do not run animation frames; don't let the layout stall there
        const later = document.visibilityState === 'visible'
            ? requestAnimationFrame
            : (cb: () => void) => setTimeout(cb, 16);
        later(apply);
    }

    async function apply() {
        scheduled = false;
        const snapshot = latest!;
        images = snapshot.items;
        height = snapshot.meta?.height ?? 0;

        if (snapshot.scrollTo !== undefined && snapshot.meta) {
            anchorState.appliedRev = snapshot.meta.rev;
            // Scroll only after the new height is in the DOM: the browser clamps
            // scrollTo to the current document height (a narrower window = taller page)
            await tick();
            window.scrollTo({top: containerTop + snapshot.scrollTo, behavior: 'instant'});
        }
    }

    // Wave on relayout: tiles that stay in their row only rescale right away;
    // tiles that move to another row start one after another (visible ones only).
    // The cap keeps a window-edge drag responsive: a new layout arrives every ~33ms
    // and restarts pending delays, so they must stay short.
    const STAGGER_STEP_MS = 60;
    const STAGGER_MAX_MS = 500;

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

<svelte:window bind:scrollY bind:innerHeight/>

<div class="masonry" bind:clientWidth={$screenWidth}>
    <div class="container" bind:this={containerEl} style="height: {height}px" class:hidden={!$screenWidth}>
        {#each images as itm (itm.guid)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="image"
                 in:fade={{ duration: 300 }}
                 style={tileStyle(itm, delays.get(itm.guid) ?? 0)}
                 onclick={() => {
                     selectItem(itm);
                     openItem(itm);
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
