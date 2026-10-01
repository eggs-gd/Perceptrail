<script lang="ts">
    import {fade} from 'svelte/transition';
    import {tick, untrack} from 'svelte';
    import {MediaQuery} from 'svelte/reactivity';
    import ItemView from "./components/ItemView.svelte";
    import GalleryTools from "./components/GalleryTools.svelte";
    import SidePanel from "./components/SidePanel.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {setOrder, updateLayout} from "$lib/workers";
    import {perceptors} from "./perceptors.svelte";
    import type {LayoutSize} from "$lib/stores";
    import {layoutDb} from "$lib/stores";
    import {type AnchorState, findAnchor, watchSize, watchWindow, type WindowSnapshot} from "./layoutWindow";

    interface Props {
        gutter?: number;
        /** Target row height, px */
        rowHeight?: number;
        openItem: (item: LayoutItem) => void,
        /** The item open in the viewer (its order); undefined: the viewer is closed */
        viewing?: number;
    }

    let {
        gutter = 8,
        rowHeight = 220,
        openItem,
        viewing,
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
    let containerTop = $state(0);
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
            if (!anchorState.pending) anchorState.pending = anchorOnScreen(width);
            updateLayout(width, targetRowHeight, anchorState.pending?.guid);
        });
    });

    /** What to keep in place: the page's edge at an edge, else the photo in the middle */
    function anchorOnScreen(width: number) {
        // The parent, not the container: the container itself may be mid-animation
        if (containerEl?.parentElement) {
            containerTop = containerEl.parentElement.getBoundingClientRect().top + window.scrollY;
        }
        const atTop = window.scrollY - containerTop <= 1;
        const atBottom = window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 2;
        return findAnchor(
            images,
            {top: window.scrollY - containerTop, height: window.innerHeight, width},
            atTop ? 'top' : atBottom ? 'bottom' : null,
        );
    }

    // A perceptor switch: the sheet is rearranged around the photo the user is at,
    // kept in place like a resize keeps it — the photo in the middle of the screen,
    // or the one the viewer was showing (then centred)
    $effect(() => {
        const request = perceptors.request;
        if (!request) return;
        untrack(() => {
            anchorState.pending = request.anchor
                ? {mode: 'center', guid: request.anchor, ratio: 0.5}
                : anchorOnScreen(screenWidth);
            setOrder(request.perceptor, anchorState.pending?.guid);
        });
    });

    // A scroll that is not ours (anchor correction) is the user's: drop the anchor,
    // the next resize anchors whatever is on screen then
    let programmaticScrollY: number | undefined;

    // The viewer may have stepped far away (arrows): when it closes, the gallery shows
    // the item it closed on — scrolled to the middle if it is not on screen. By guid:
    // closing into another perceptor changes the order (that switch centres it itself).
    let lastViewed: string | undefined;
    $effect(() => {
        const order = viewing;
        if (order !== undefined) {
            layoutDb.items.where('order').equals(order).first().then((item) => {
                if (item && viewing === order) lastViewed = item.guid;
            });
        } else if (lastViewed !== undefined) {
            const guid = lastViewed;
            lastViewed = undefined;
            if (anchorState.pending?.guid !== guid) reveal(guid);
        }
    });

    async function reveal(guid: string) {
        const item = await layoutDb.items.get(guid);
        if (!item || !containerEl?.parentElement) return;
        const top = containerEl.parentElement.getBoundingClientRect().top + window.scrollY + item.y;
        if (top >= window.scrollY && top + item.h <= window.scrollY + window.innerHeight) return;
        programmaticScrollY = Math.max(0, top - (window.innerHeight - item.h) / 2);
        window.scrollTo({top: programmaticScrollY, behavior: 'instant'});
    }

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
        const relayout = meta !== undefined && meta.rev !== lastRev;
        // Tiles mounted by a relayout appear in place at once: fading them in from 0
        // left the screen nearly white when widening (the shorter layout brings in many
        // photos that were not rendered). New photos from the stream still fade in.
        fadeMs = relayout ? 0 : FADE_MS;

        // FLIP "first": where every tile is right now, mid-animation included
        const first = measureTiles();
        if (snapshot) images = snapshot.items;

        // Height: the size record grows with the stream; right after a relayout the
        // layout record may be newer than it
        height = latestSize && latestSize.rev >= (meta?.rev ?? 0)
            ? latestSize.height
            : meta?.height ?? 0;

        // New positions and height in the DOM before scrolling (the browser clamps
        // scrollTo to the current document height) and before animating
        await tick();

        let delta = 0;
        if (snapshot?.scrollTo !== undefined && meta && meta.rev !== anchorState.appliedRev) {
            anchorState.appliedRev = meta.rev;
            const before = window.scrollY;
            programmaticScrollY = snapshot.toEnd
                ? document.documentElement.scrollHeight - window.innerHeight
                : containerTop + snapshot.scrollTo;
            window.scrollTo({top: programmaticScrollY, behavior: 'instant'});
            delta = window.scrollY - before;
        }
        if (meta) lastRev = meta.rev;

        animateTiles(first, delta, relayout ? waveOrigin() : undefined);
    }

    // Tiles move to their new place with these durations (Web Animations, FLIP)
    const MOVE_MS = 500;
    const SIZE_MS = 200;
    // Wave: the anchor starts first, the others one after another outwards. The whole
    // wave over the visible photos fits into STAGGER_MAX_MS (a window-edge drag sends a
    // relayout every ~33 ms, so it must stay short); STAGGER_STEP_MS is the step limit
    // when only a few photos are visible.
    const STAGGER_STEP_MS = 60;
    const STAGGER_MAX_MS = 500;

    const reducedMotion = new MediaQuery('prefers-reduced-motion: reduce');
    let lastRev = -1;
    const FADE_MS = 300;
    let fadeMs = $state(FADE_MS);

    interface Box { x: number, y: number, w: number, h: number }

    /** Our running move/size animations per tile (not Svelte's fade-in) */
    const moves = new WeakMap<Element, Animation[]>();

    /** Current visual box of every rendered tile, mid-animation included */
    function measureTiles(): Map<string, Box> {
        const boxes = new Map<string, Box>();
        if (!containerEl || reducedMotion.current) return boxes;
        for (const el of containerEl.querySelectorAll<HTMLElement>('[data-guid]')) {
            const style = getComputedStyle(el);
            const m = new DOMMatrix(style.transform);
            boxes.set(el.dataset.guid!, {x: m.m41, y: m.m42, w: parseFloat(style.width), h: parseFloat(style.height)});
        }
        return boxes;
    }

    /** Where the relayout wave starts: a row, and a point in it (x) */
    interface WaveOrigin { row: number, x: number }

    /**
     * Pinned photo mid-page: its row, from the photo outwards. Page top: the first
     * visible row, page bottom: the last visible row — both from the left edge.
     */
    function waveOrigin(): WaveOrigin | undefined {
        const a = anchorState.pending;
        if (a?.guid) {
            const pinned = images.find((i) => i.guid === a.guid);
            if (pinned) return {row: pinned.row, x: pinned.x + pinned.w / 2};
        }
        const top = window.scrollY - containerTop;
        const visible = images.filter((i) => i.y + i.h > top && i.y < top + window.innerHeight);
        if (visible.length === 0) return undefined;
        const rows = visible.map((i) => i.row);
        return {row: a?.mode === 'bottom' ? Math.max(...rows) : Math.min(...rows), x: 0};
    }

    /**
     * The wave goes by rows: rows at the same distance above and below the origin start
     * together, and within a row photos go left to right (in the origin row: from the
     * origin outwards). Reverse photo order — right to left, bottom to top — looked
     * unnatural going up. Returns delays for the photos visible before or after the
     * relayout, the whole wave fitting into STAGGER_MAX_MS.
     */
    function waveDelays(origin: WaveOrigin, first: Map<string, Box>, delta: number): Map<string, number> {
        const top = window.scrollY - containerTop;
        const bottom = top + window.innerHeight;
        const involved = images.filter((i) => {
            const from = first.get(i.guid);
            const seenNow = i.y + i.h > top && i.y < bottom;
            const seenBefore = from !== undefined && from.y + delta + from.h > top && from.y + delta < bottom;
            return seenNow || seenBefore;
        });

        const rows = new Map<number, LayoutItem[]>();
        for (const i of involved) (rows.get(i.row) ?? rows.set(i.row, []).get(i.row)!).push(i);

        // Position in the wave: whole rows by distance + the photo's place in its row
        const position = new Map<string, number>();
        let last = 0;
        for (const [row, items] of rows) {
            const distance = Math.abs(row - origin.row);
            const ordered = distance === 0
                ? [...items].sort((a, b) => Math.abs(a.x + a.w / 2 - origin.x) - Math.abs(b.x + b.w / 2 - origin.x))
                : [...items].sort((a, b) => a.x - b.x);
            ordered.forEach((itm, k) => {
                const p = distance + k / ordered.length;
                position.set(itm.guid, p);
                last = Math.max(last, p);
            });
        }

        const avgRow = involved.length / Math.max(1, rows.size);
        const rowMs = Math.min(STAGGER_STEP_MS * avgRow, STAGGER_MAX_MS / Math.max(1, last));
        const delays = new Map<string, number>();
        for (const [guid, p] of position) delays.set(guid, Math.min(p * rowMs, STAGGER_MAX_MS));
        return delays;
    }

    /**
     * FLIP per tile: from where it is on screen now to its new place. `delta` is the
     * scroll correction just applied — the start is shifted by it so nothing jumps on
     * screen. A tile waiting for its turn in the wave stays exactly where it was
     * (fill: backwards). New tiles have no start and just fade in.
     */
    function animateTiles(first: Map<string, Box>, delta: number, origin: WaveOrigin | undefined) {
        if (!containerEl || first.size === 0) return;
        const byGuid = new Map(images.map((i) => [i.guid, i]));
        const delays = origin ? waveDelays(origin, first, delta) : undefined;

        for (const el of containerEl.querySelectorAll<HTMLElement>('[data-guid]')) {
            const from = first.get(el.dataset.guid!);
            const to = byGuid.get(el.dataset.guid!);
            if (!from || !to) continue;
            const fromY = from.y + delta;
            if (Math.abs(from.x - to.x) < 0.5 && Math.abs(fromY - to.y) < 0.5
                && Math.abs(from.w - to.w) < 0.5 && Math.abs(from.h - to.h) < 0.5) continue;

            moves.get(el)?.forEach((a) => a.cancel());
            const delay = delays?.get(to.guid) ?? 0;
            moves.set(el, [
                el.animate(
                    [{transform: `translate(${from.x}px, ${fromY}px)`}, {transform: `translate(${to.x}px, ${to.y}px)`}],
                    {duration: MOVE_MS, delay, easing: 'ease', fill: 'backwards'},
                ),
                el.animate(
                    [{width: `${from.w}px`, height: `${from.h}px`}, {width: `${to.w}px`, height: `${to.h}px`}],
                    {duration: SIZE_MS, delay, easing: 'ease', fill: 'backwards'},
                ),
            ]);
        }
    }

    // Positions come from the layout worker. Tiles are absolutely positioned and
    // keyed by guid, so on relayout (resize) the same DOM node moves to its new
    // place — also into another row — animated by animateTiles().
    function tileStyle(itm: LayoutItem) {
        return `transform: translate(${itm.x}px, ${itm.y}px); width: ${itm.w}px; height: ${itm.h}px;`;
    }

</script>

<svelte:window bind:scrollY bind:innerHeight onscroll={onScroll}/>

<!-- The pinned side panel takes its width from the photos (a resize: the photo in
     the middle stays in place); unpinned it shows over them while scrolling -->
<div class="masonry" class:pinned={perceptors.pinned} bind:clientWidth={screenWidth}>
    <div class={['container', !screenWidth && 'hidden']} bind:this={containerEl} style:height="{height}px">
        {#each images as itm (itm.guid)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="image"
                 data-guid={itm.guid}
                 in:fade={{ duration: fadeMs }}
                 style={tileStyle(itm)}
                 onclick={() => {
                     openItem(itm);
                 }}>
                <ItemView item={itm} index={itm.order} sizes="{Math.round(itm.w)}px"/>
            </div>
        {/each}
    </div>
</div>
{#if viewing === undefined}
    <GalleryTools/>
    <SidePanel {height} top={containerTop} {scrollY} {innerHeight} pinned={perceptors.pinned}/>
{/if}

<style>
    .masonry {
        max-width: 100%;
        /* We anchor the view ourselves (resize). Browser scroll anchoring would adjust
           scrollY on its own, which reads as a user scroll and drops our anchor. */
        overflow-anchor: none;
    }

    /* The pinned side panel's width (SidePanel) */
    .masonry.pinned {
        margin-right: 4.5rem;
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
    }

    .image > :global(*) {
        width: 100%;
        height: 100%;
    }

    .hidden {
        visibility: hidden;
    }

</style>
