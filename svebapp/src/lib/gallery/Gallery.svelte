<script lang="ts">
    import {fade} from 'svelte/transition';
    import {tick, untrack} from 'svelte';
    import {MediaQuery} from 'svelte/reactivity';
    import ItemView from "./components/ItemView.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {updateLayout} from "$lib/workers";
    import type {LayoutSize} from "$lib/stores";
    import {type AnchorState, findAnchor, watchSize, watchWindow, type WindowSnapshot} from "./layoutWindow";

    interface Props {
        gutter?: number;
        /** Target row height, px */
        rowHeight?: number;
        openItem: (item: LayoutItem) => void,
    }

    let {
        gutter = 8,
        rowHeight = 220,
        openItem
    }: Props = $props();

    /** Width available to the gallery (bound to the container) */
    let screenWidth = $state(0);

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

    // Width or row height changed: ask for a relayout that reports where the anchor
    // (the item at the top of the viewport) ends up. The anchor is taken once and kept
    // until the user scrolls — also across bursts of resizes: taken anew each time it
    // would walk the view (while a relayout is on its way the screen shows the old
    // layout; after a reflow the top-left photo is often an earlier one).
    $effect(() => {
        const width = screenWidth;
        const targetRowHeight = rowHeight;
        if (!width) return;
        untrack(() => {
            if (!anchorState.pending) {
                // The parent, not the container: the container itself may be mid-animation
                if (containerEl?.parentElement) {
                    containerTop = containerEl.parentElement.getBoundingClientRect().top + window.scrollY;
                }
                const atTop = window.scrollY - containerTop <= 1;
                const atBottom = window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 2;
                anchorState.pending = findAnchor(
                    images,
                    {top: window.scrollY - containerTop, height: window.innerHeight, width},
                    atTop ? 'top' : atBottom ? 'bottom' : null,
                );
            }
            updateLayout(width, targetRowHeight, anchorState.pending?.guid);
        });
    });

    // A scroll that is not ours (anchor correction) is the user's: drop the anchor,
    // the next resize anchors whatever is on screen then
    let programmaticScrollY: number | undefined;

    function onScroll() {
        if (programmaticScrollY !== undefined && Math.abs(window.scrollY - programmaticScrollY) <= 2) return;
        programmaticScrollY = undefined;
        anchorState.pending = undefined;
    }

    $effect(() => {
        const sub = watchWindow(range, anchorState).subscribe({
            next: (snapshot) => { latestWindow = snapshot; schedule(); },
            error: (error) => console.error('layout window', error),
        });
        return () => sub.unsubscribe();
    });

    $effect(() => {
        const sub = watchSize().subscribe({
            next: (size) => { latestSize = size; schedule(); },
            error: (error) => console.error('layout size', error),
        });
        return () => sub.unsubscribe();
    });

    // Apply at most once per frame: during the stream both subscriptions fire often
    let latestWindow: WindowSnapshot | undefined;
    let latestSize: LayoutSize | undefined;
    let scheduled = false;

    function schedule() {
        if (scheduled) return;
        scheduled = true;
        // Next frame; the timer covers windows that are "visible" but not painting
        // (occluded, throttled), where animation frames stall
        requestAnimationFrame(apply);
        setTimeout(apply, 100);
    }

    async function apply() {
        if (!scheduled) return; // the other trigger already ran
        scheduled = false;
        const snapshot = latestWindow;
        const meta = snapshot?.meta;
        if (snapshot) images = snapshot.items;

        // Height: the size record grows with the stream; right after a relayout the
        // layout record may be newer than it
        height = latestSize && latestSize.rev >= (meta?.rev ?? 0)
            ? latestSize.height
            : meta?.height ?? 0;

        if (snapshot?.scrollTo !== undefined && meta && meta.rev !== anchorState.appliedRev) {
            anchorState.appliedRev = meta.rev;
            // Scroll only after the new height is in the DOM: the browser clamps
            // scrollTo to the current document height (a narrower window = taller page)
            await tick();
            const before = window.scrollY;
            programmaticScrollY = snapshot.toEnd
                ? document.documentElement.scrollHeight - window.innerHeight
                : containerTop + snapshot.scrollTo;
            window.scrollTo({top: programmaticScrollY, behavior: 'instant'});
            compensateScroll(window.scrollY - before);
        }
    }

    // Tiles move to their new place with this duration (CSS below uses --move-ms)
    const MOVE_MS = 500;
    const reducedMotion = new MediaQuery('prefers-reduced-motion: reduce');
    let shift: Animation | undefined;

    /**
     * FLIP for the scroll correction. Tiles animate in page coordinates while the anchor
     * correction scrolls the page by `delta` at once — on screen everything would jump
     * by delta and fly back. Shifting the container by the same delta and animating that
     * shift to 0 alongside the tiles keeps the pinned photo still for the whole
     * animation; everything else moves relative to it. A new correction mid-animation
     * starts from the current shift, so a window-edge drag stays continuous.
     */
    function compensateScroll(delta: number) {
        if (!delta || !containerEl || reducedMotion.current) return;
        const current = shift && shift.playState !== 'finished'
            ? new DOMMatrix(getComputedStyle(containerEl).transform).m42
            : 0;
        shift?.cancel();
        shift = containerEl.animate(
            [{transform: `translateY(${current + delta}px)`}, {transform: 'translateY(0)'}],
            {duration: MOVE_MS, easing: 'ease'},
        );
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

<svelte:window bind:scrollY bind:innerHeight onscroll={onScroll}/>

<div class="masonry" bind:clientWidth={screenWidth}>
    <div class={['container', !screenWidth && 'hidden']} bind:this={containerEl} style:height="{height}px" style:--move-ms="{MOVE_MS}ms">
        {#each images as itm (itm.guid)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="image"
                 in:fade={{ duration: 300 }}
                 style={tileStyle(itm, delays.get(itm.guid) ?? 0)}
                 onclick={() => {
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
        /* We anchor the view ourselves (resize). Browser scroll anchoring would adjust
           scrollY on its own, which reads as a user scroll and drops our anchor. */
        overflow-anchor: none;
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
            transform var(--move-ms) ease,
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
