<script lang="ts">
    import PerceptorButtons from './PerceptorButtons.svelte';
    import {perceptors, togglePinned} from '../perceptors.svelte';

    interface Props {
        /** A view was picked (the gallery keeps the photo in the middle in place) */
        onpick: (slug: string) => void;
    }

    let {onpick}: Props = $props();

    // The gallery's toolbar: the perceptors (the sheet's views) and the side panel's pin
    let pinTitle = $derived(perceptors.pinned ? 'Side panel: always shown' : 'Side panel: shown while scrolling');
</script>

<div class="tools" role="toolbar">
    <PerceptorButtons {onpick}/>
    <button class="tool" class:on={perceptors.pinned} title={pinTitle} aria-label={pinTitle}
            aria-pressed={perceptors.pinned} onclick={togglePinned}>
        <svg viewBox="0 0 24 24" aria-hidden="true">
            <rect x="3.5" y="4.5" width="17" height="15" rx="2" fill="none" stroke="currentColor" stroke-width="1.6"/>
            <path d="M15 4.5v15" stroke="currentColor" stroke-width="1.6"/>
            <path d="M17 8.5h1.5M17 12h1.5M17 15.5h1.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
        </svg>
    </button>
</div>

<style>
    .tools {
        position: fixed;
        top: 1rem;
        right: 1rem;
        z-index: 2;
        display: flex;
        gap: 0.5rem;
    }

    .tool {
        display: flex;
        padding: 0.45rem;
        border: 1px solid transparent;
        border-radius: 50%;
        background: rgb(0 0 0 / 0.55);
        color: rgb(255 255 255 / 0.7);
        cursor: pointer;
    }

    .tool:hover {
        color: #fff;
    }

    .tool.on {
        border-color: rgb(255 255 255 / 0.8);
        color: #fff;
    }

    svg {
        width: 20px;
        height: 20px;
    }
</style>
