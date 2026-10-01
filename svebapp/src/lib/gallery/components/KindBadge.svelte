<script lang="ts">
    import type {Asset, AssetKind} from '$lib/stores';
    import {formatDuration, hasLocalVideo} from './asset';

    interface Props {
        asset: Asset;
    }

    let {asset}: Props = $props();

    const TITLES: Record<AssetKind, string> = {photo: '', live: 'Live Photo', video: 'Video'};

    // Its video is only in iCloud: hover shows a few frames at best, nothing plays
    let cloud = $derived(asset.kind !== 'photo' && !hasLocalVideo(asset));
    let title = $derived(TITLES[asset.kind] + (cloud ? ' (in iCloud only)' : ''));
</script>

<!-- A moving asset is marked on the tile; a photo has no mark -->
{#if asset.kind !== 'photo'}
    <span class="badge kind" {title} aria-label={title}>
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
        {#if cloud}
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M7 18.5h10.5a4 4 0 0 0 .6-7.95A6 6 0 0 0 6.6 9.1 4.7 4.7 0 0 0 7 18.5z"
                      fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/>
            </svg>
        {/if}
    </span>
    {#if asset.kind === 'video' && asset.duration}
        <span class="badge duration">{formatDuration(asset.duration)}</span>
    {/if}
{/if}

<style>
    .badge {
        position: absolute;
        display: flex;
        align-items: center;
        gap: 2px;
        padding: 3px;
        border-radius: 10px;
        background: rgb(0 0 0 / 0.45);
        color: #fff;
        pointer-events: none;
    }

    .kind {
        top: 6px;
        left: 6px;
    }

    .duration {
        right: 6px;
        bottom: 6px;
        padding: 1px 6px;
        font: 600 11px/16px system-ui, sans-serif;
        font-variant-numeric: tabular-nums;
    }

    svg {
        width: 16px;
        height: 16px;
    }
</style>
