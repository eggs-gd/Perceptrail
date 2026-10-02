<script lang="ts">
    import type {Asset, AssetKind} from '$lib/stores';
    import {formatDuration} from './asset';

    interface Props {
        asset: Asset;
        /** A video's length, seconds: the asset's, unless the caller knows better */
        duration?: number;
        /** Its video is on its way: a ring turns around the mark */
        loading?: boolean;
    }

    let {asset, duration = asset.duration, loading = false}: Props = $props();

    const TITLES: Record<AssetKind, string> = {photo: 'Photo', live: 'Live Photo', video: 'Video'};

    // The original is only in iCloud — the same for every kind: what is shown or
    // played comes from Photos on demand, the cloud says where the original is
    // (a Live Photo's original is its video)
    let cloud = $derived(!asset.original);
    let title = $derived(TITLES[asset.kind] + (cloud ? ' (in iCloud only)' : ''));
</script>

<!-- What moves and what is only in iCloud is marked; a local photo has no mark -->
{#if asset.kind !== 'photo' || cloud}
    <span class="badge" {title} aria-label={title}>
        <!-- The cloud first, then the kind and a video's length -->
        {#if cloud}
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M7 18.5h10.5a4 4 0 0 0 .6-7.95A6 6 0 0 0 6.6 9.1 4.7 4.7 0 0 0 7 18.5z"
                      fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/>
            </svg>
        {/if}
        {#if asset.kind !== 'photo'}
            <span class="mark">
            {#if loading}
                <svg class="spinner" viewBox="0 0 24 24" aria-hidden="true">
                    <circle cx="12" cy="12" r="11" fill="none" stroke="currentColor" stroke-width="2"
                            stroke-linecap="round" stroke-dasharray="17 52"/>
                </svg>
            {/if}
            <svg viewBox="0 0 24 24" aria-hidden="true">
                {#if asset.kind === 'live'}
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
        {#if asset.kind === 'video' && duration}
            <span class="duration">{formatDuration(duration)}</span>
        {/if}
    </span>
{/if}

<style>
    .badge {
        position: absolute;
        top: 6px;
        left: 6px;
        display: flex;
        align-items: center;
        gap: 2px;
        padding: 3px;
        border-radius: 10px;
        background: rgb(0 0 0 / 0.45);
        color: #fff;
        pointer-events: none;
    }

    .duration {
        padding: 0 2px;
        font: 600 11px/16px system-ui, sans-serif;
        font-variant-numeric: tabular-nums;
    }

    svg {
        width: 16px;
        height: 16px;
    }

    .mark {
        position: relative;
        display: flex;
    }

    /* Around the mark while its video comes (as Immich and Google Photos do) —
       only if it takes a while: a video that starts at once (here already, or in the
       browser's cache) would make it blink for a frame or two */
    .spinner {
        position: absolute;
        inset: -3px;
        width: 22px;
        height: 22px;
        opacity: 0;
        animation: appear 0s 300ms forwards, spin 0.9s linear infinite;
    }

    @keyframes appear {
        to {
            opacity: 1;
        }
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
</style>
