<script lang="ts">
    import type {Asset, AssetKind} from '$lib/stores';
    import {formatDuration, hasLocalVideo} from './asset';

    interface Props {
        asset: Asset;
    }

    let {asset}: Props = $props();

    const TITLES: Record<AssetKind, string> = {photo: 'Photo', live: 'Live Photo', video: 'Video'};

    // Only in iCloud: a photo whose original is not here (we show Photos' preview), a
    // video or a Live Photo whose video is not here (it does not play)
    let cloud = $derived(asset.kind === 'photo' ? !asset.original : !hasLocalVideo(asset));
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
        {/if}
        {#if asset.kind === 'video' && asset.duration}
            <span class="duration">{formatDuration(asset.duration)}</span>
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
</style>
