<script lang="ts">
    import type {AssetKind} from '$lib/stores';

    interface Props {
        kind: AssetKind;
    }

    let {kind}: Props = $props();

    const TITLES: Record<AssetKind, string> = {photo: '', live: 'Live Photo', video: 'Video'};
</script>

<!-- A moving asset is marked on the tile; a photo has no mark -->
{#if kind !== 'photo'}
    <span class="badge" title={TITLES[kind]} aria-label={TITLES[kind]}>
        <svg viewBox="0 0 24 24" aria-hidden="true">
            {#if kind === 'live'}
                <!-- Live Photo: a dot in a ring in a dashed ring -->
                <circle cx="12" cy="12" r="3" fill="currentColor"/>
                <circle cx="12" cy="12" r="6.5" fill="none" stroke="currentColor" stroke-width="1.6"/>
                <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="1.6"
                        stroke-dasharray="1.6 2.4"/>
            {:else}
                <path d="M8 5.5v13l11-6.5z" fill="currentColor"/>
            {/if}
        </svg>
    </span>
{/if}

<style>
    .badge {
        position: absolute;
        top: 6px;
        left: 6px;
        display: flex;
        padding: 3px;
        border-radius: 50%;
        background: rgb(0 0 0 / 0.45);
        color: #fff;
        pointer-events: none;
        transition: opacity 150ms;
    }

    svg {
        width: 16px;
        height: 16px;
    }
</style>
