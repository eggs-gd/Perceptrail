<script lang="ts">
    import type {Asset} from '$lib/stores';
    import {assetUrl, canShowImage, hasImage, originalImage, playableVideos} from './asset';
    import {toggleViewerPref, viewerPrefs} from './viewerPrefs.svelte';
    import PerceptorButtons from './PerceptorButtons.svelte';

    interface Props {
        asset: Asset;
        /** The original is shown in place of the preview */
        showOriginal: boolean;
        ontoggleoriginal: () => void;
        /** A view was picked: to the sheet in its order, around this photo */
        onperceptor: (slug: string) => void;
        /** The info panel is shown */
        infoOpen: boolean;
        ontoggleinfo: () => void;
    }

    let {asset, showOriginal, ontoggleoriginal, onperceptor, infoOpen, ontoggleinfo}: Props = $props();

    // What the viewer can do with this asset decides the buttons
    let playable = $derived(playableVideos(asset).length > 0);
    // Apple Photos: a video or a motion not here yet is asked for when it plays
    let live = $derived(asset.kind === 'live' && (playable || !!asset.onDemand?.hover));
    // A playable video is shown as itself: no switch; one the browser cannot play
    // is downloaded
    let video = $derived(asset.kind === 'video' && (playable || !!asset.onDemand));
    let image = $derived(video ? undefined : originalImage(asset));
    let other = $derived(!video && !image ? asset.original : null);
    // A photo or a video is shown: its size can be stretched
    let still = $derived(video || hasImage(asset));

    const tooltips = {
        live: () => (viewerPrefs.autoplayLive ? 'Live Photo plays on open' : 'Live Photo does not play on open'),
        video: () => (viewerPrefs.autoplayVideo ? 'Video plays on open' : 'Video does not play on open'),
        stretch: () => (viewerPrefs.stretchSmall ? 'Small photos and videos fill the screen' : 'Small photos and videos at their own size'),
    };
</script>

{#snippet download(url: string)}
    <a class="tool" href={url} download title="Download original" aria-label="Download original">
        <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 19.5h14" fill="none" stroke="currentColor"
                  stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
    </a>
{/snippet}

{#snippet slash(on: boolean)}
    {#if !on}
        <path d="M4 4l16 16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
    {/if}
{/snippet}

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="tools" role="toolbar" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <PerceptorButtons onpick={onperceptor}/>
    <span class="divider"></span>
    {#if live}
        <button class="tool" class:on={viewerPrefs.autoplayLive} title={tooltips.live()} aria-label={tooltips.live()}
                aria-pressed={viewerPrefs.autoplayLive} onclick={() => toggleViewerPref('autoplayLive')}>
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r="3" fill="currentColor"/>
                <circle cx="12" cy="12" r="6.5" fill="none" stroke="currentColor" stroke-width="1.6"/>
                <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="1.6"
                        stroke-dasharray="1.6 2.4"/>
                {@render slash(viewerPrefs.autoplayLive)}
            </svg>
        </button>
    {/if}
    {#if video}
        <button class="tool" class:on={viewerPrefs.autoplayVideo} title={tooltips.video()} aria-label={tooltips.video()}
                aria-pressed={viewerPrefs.autoplayVideo} onclick={() => toggleViewerPref('autoplayVideo')}>
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r="9.5" fill="none" stroke="currentColor" stroke-width="1.6"/>
                <path d="M10 8.2v7.6l6-3.8z" fill="currentColor"/>
                {@render slash(viewerPrefs.autoplayVideo)}
            </svg>
        </button>
    {/if}
    <button class="tool" class:on={infoOpen} title="Info" aria-label="Info" aria-pressed={infoOpen}
            onclick={ontoggleinfo}>
        <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="12" r="9.5" fill="none" stroke="currentColor" stroke-width="1.6"/>
            <path d="M12 11v6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
            <circle cx="12" cy="7.6" r="1.1" fill="currentColor"/>
        </svg>
    </button>
    {#if still}
        <button class="tool" class:on={viewerPrefs.stretchSmall} title={tooltips.stretch()}
                aria-label={tooltips.stretch()} aria-pressed={viewerPrefs.stretchSmall}
                onclick={() => toggleViewerPref('stretchSmall')}>
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M14 4h6v6M10 20H4v-6M20 4l-6.5 6.5M4 20l6.5-6.5" fill="none" stroke="currentColor"
                      stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
        </button>
    {/if}
    {#if image}
        <!-- Tested when the item opens: the switch if the browser shows it, else a download -->
        {#await canShowImage(image) then shown}
            {#if shown}
                <button class="tool" class:on={showOriginal}
                        title={showOriginal ? 'Showing the original' : 'Show the original'}
                        aria-label="Original" aria-pressed={showOriginal} onclick={ontoggleoriginal}>
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                        <rect x="3.5" y="5" width="17" height="14" rx="2" fill="none" stroke="currentColor"
                              stroke-width="1.6"/>
                        <path d="M3.5 16.5l5-5 4 4 2.5-2.5 5.5 5.5" fill="none" stroke="currentColor"
                              stroke-width="1.6" stroke-linejoin="round"/>
                        <circle cx="15.5" cy="9.5" r="1.6" fill="currentColor"/>
                    </svg>
                </button>
            {:else}
                {@render download(assetUrl(image))}
            {/if}
        {/await}
    {:else if other}
        {@render download(assetUrl(other))}
    {/if}
</div>

<style>
    .tools {
        position: fixed;
        top: 1rem;
        right: 1rem;
        z-index: 1;
        /* The layout turns pointer events off under the viewer (main.viewing): on again */
        pointer-events: auto;
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

    .divider {
        width: 1px;
        margin: 0.3rem 0.1rem;
        background: rgb(255 255 255 / 0.3);
    }

    svg {
        width: 20px;
        height: 20px;
    }
</style>
