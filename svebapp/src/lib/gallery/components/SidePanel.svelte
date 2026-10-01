<script lang="ts">
    import {LAYOUT_SECTIONS_KEY, layoutDb, type LayoutSections, type SectionMark} from '$lib/stores';
    // Not re-exported from $lib/stores: the workers import that, this is page-only
    import {LiveQuery} from '$lib/stores/internal/liveQuery';
    import {sheetScale} from './sheetScale';

    interface Props {
        /** Gallery height and where it starts on the page, px; its photo count */
        height: number;
        count: number;
        top: number;
        scrollY: number;
        innerHeight: number;
        /** Always shown (and the photos make room); else it shows up while scrolling */
        pinned: boolean;
    }

    let {height, count, top, scrollY, innerHeight, pinned}: Props = $props();

    // The side panel: the active perceptor's sections along the whole sheet (years and
    // months for the date), where the view is, and a scrubber — press or drag to jump

    const MIN_GAP = 16;        // px between shown labels
    const SHOW_AFTER_MS = 1200; // an unpinned panel stays this long after a scroll

    const sectionsQuery = new LiveQuery(() => layoutDb.meta.get(LAYOUT_SECTIONS_KEY) as Promise<LayoutSections | undefined>);
    let marks = $derived(sectionsQuery.current?.marks ?? []);

    let trackHeight = $state(0);
    let pointerY = $state<number | undefined>(); // over the track, px from its top
    let dragging = $state(false);
    let scrolling = $state(false);

    // Track position ↔ gallery y: by sections, each by √(its photos) — see sheetScale
    let scale = $derived(sheetScale(marks, height, count, trackHeight, MIN_GAP));
    const toTrack = (y: number) => scale.toTrack(y);
    const toSheet = (p: number) => scale.toSheet(p);

    // Labels that fit: every coarsest one that does not touch the previous, deeper ones
    // only where there is room around them
    let shown = $derived.by(() => {
        const out: (SectionMark & {at: number})[] = [];
        for (const level of [0, 1, 2]) {
            for (const m of marks) {
                if (m.level !== level) continue;
                const at = toTrack(m.y);
                if (out.every((o) => Math.abs(o.at - at) >= MIN_GAP)) out.push({...m, at});
            }
        }
        return out.sort((a, b) => a.at - b.at);
    });

    // The view on the track: taller where photos are sparse, shorter where dense
    let viewTop = $derived(toTrack(Math.max(0, scrollY - top)));
    let viewHeight = $derived(Math.max(2, toTrack(Math.max(0, scrollY - top) + innerHeight) - viewTop));

    // The section under the pointer with the sections it is in ("September 2025"):
    // per level, the last one starting above it, down to the deepest found
    let hovered = $derived.by(() => {
        if (pointerY === undefined) return undefined;
        const y = toSheet(pointerY);
        const byLevel: SectionMark[] = [];
        for (const m of marks) {
            if (m.y > y) continue;
            const prev = byLevel[m.level];
            if (!prev || m.y >= prev.y) byLevel[m.level] = m;
        }
        // A deeper section counts only inside the coarser one found
        const path: string[] = [];
        let from = -Infinity;
        for (const m of byLevel) {
            if (!m || m.y < from) break;
            path.unshift(m.label);
            from = m.y;
        }
        return path.length ? path.join(' ') : undefined;
    });

    // Unpinned: shown while scrolling (and a moment after), while pointed at or dragged
    let hideTimer: ReturnType<typeof setTimeout> | undefined;
    function onscroll() {
        scrolling = true;
        clearTimeout(hideTimer);
        hideTimer = setTimeout(() => (scrolling = false), SHOW_AFTER_MS);
    }
    let visible = $derived(pinned || scrolling || dragging || pointerY !== undefined);

    function jump(clientY: number, track: HTMLElement) {
        const p = Math.min(trackHeight, Math.max(0, clientY - track.getBoundingClientRect().top));
        // The pointed place in the middle of the screen
        window.scrollTo({top: Math.max(0, top + toSheet(p) - innerHeight / 2), behavior: 'instant'});
    }
</script>

<svelte:window {onscroll}/>

<!-- The shade covers the whole height; the track (the sheet's scale) sits inside it,
     below the toolbar -->
<div class={['panel', pinned && 'pinned', visible && 'visible']}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="track"
         onpointerdown={(e) => {
             dragging = true;
             e.currentTarget.setPointerCapture(e.pointerId);
             jump(e.clientY, e.currentTarget);
         }}
         onpointermove={(e) => {
             pointerY = e.clientY - e.currentTarget.getBoundingClientRect().top;
             if (dragging) jump(e.clientY, e.currentTarget);
         }}
         onpointerup={() => (dragging = false)}
         onpointercancel={() => (dragging = false)}
         onpointerleave={() => { if (!dragging) pointerY = undefined; }}
         bind:clientHeight={trackHeight}>
        <div class="view" style:top="{viewTop}px" style:height="{viewHeight}px"></div>
        {#each shown as m (`${m.guid}:${m.level}`)}
            <span class={['mark', `level${m.level}`]} style:top="{m.at}px">{m.label}</span>
        {/each}
        {#if pointerY !== undefined}
            <div class="pointer" style:top="{pointerY}px">
                {#if hovered}<span class="tip">{hovered}</span>{/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .panel {
        position: fixed;
        top: 0;
        right: 0;
        bottom: 0;
        z-index: 1;
        width: 4.5rem;
        opacity: 0;
        pointer-events: none;
        transition: opacity 200ms;
    }

    .track {
        position: absolute;
        top: 4.5rem;
        right: 0;
        bottom: 1rem;
        left: 0;
        cursor: ns-resize;
        touch-action: none;
        user-select: none;
    }

    .panel.visible {
        opacity: 1;
        pointer-events: auto;
    }

    /* Over the photos: a shade so the labels read on any photo */
    .panel:not(.pinned) {
        background: linear-gradient(to left, rgb(0 0 0 / 0.55), rgb(0 0 0 / 0));
    }

    .view {
        position: absolute;
        right: 0;
        width: 3px;
        border-radius: 2px;
        background: rgb(255 255 255 / 0.7);
    }

    .mark {
        position: absolute;
        right: 0.6rem;
        transform: translateY(-50%);
        font: 0.7rem/1 system-ui, sans-serif;
        color: rgb(255 255 255 / 0.6);
        white-space: nowrap;
        pointer-events: none;
    }

    .mark.level0 {
        font-weight: 600;
        color: rgb(255 255 255 / 0.9);
    }

    .pointer {
        position: absolute;
        right: 0;
        left: 0;
        border-top: 1px solid rgb(255 255 255 / 0.8);
        pointer-events: none;
    }

    .tip {
        position: absolute;
        right: calc(100% + 0.4rem);
        transform: translateY(-50%);
        padding: 0.2rem 0.5rem;
        border-radius: 0.3rem;
        background: rgb(0 0 0 / 0.75);
        color: #fff;
        font: 0.8rem system-ui, sans-serif;
        white-space: nowrap;
    }
</style>
