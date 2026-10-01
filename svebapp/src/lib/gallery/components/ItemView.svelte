<script lang="ts">
    import {type Item} from '$lib/stores'
    import type {Attachment} from 'svelte/attachments';

    import Img from "./Img.svelte";
    import Video from "./Video.svelte";
    import Picture from "./Picture.svelte";
    import Motion from "./Motion.svelte";
    import KindBadge from "./KindBadge.svelte";
    import {assetUrl, biggestImage, fallbackImage, hasImage, originalImage, playableVideos} from "./asset";
    import {viewerPrefs} from "./viewerPrefs.svelte";

    interface Props {
        item: Item;
        index: number;
        /** tile: the gallery (motion on hover); view: the viewer (full size, the original) */
        mode?: 'tile' | 'view';
        /** How wide it is shown: the browser picks the size of the image by it */
        sizes?: string;
        /** The viewer: show the original image in place of the preview (ViewerTools) */
        showOriginal?: boolean;
    }

    let {item, index, mode = 'tile', sizes = '100vw', showOriginal = false}: Props = $props();

    // The client decides what to show: the asset has every file by role
    let asset = $derived(item.asset);
    // An original the browser shows counts too (a generic PNG/JPEG with no smaller copy)
    let hasStill = $derived(!!asset && hasImage(asset));
    let videos = $derived(asset ? playableVideos(asset) : []);
    let hasMotion = $derived(videos.length > 0 || (asset?.frames.length ?? 0) > 1);
    let hovered = $state(false);

    // A video with no image (a plain .mp4 in a folder): the tile is the video itself,
    // paused on its first frame and played on hover. #t: Safari paints no frame of a
    // paused video until it is asked for a time. Its length comes from the file
    // when the server has none (an item processed before durations existed).
    let fileDuration = $state<number>();
    // Re-runs when hovered changes: plays on hover, pauses after
    const playOnHover: Attachment<HTMLVideoElement> = (video) => {
        if (hovered) video.play().catch(() => {});
        else video.pause();
    };

    // The viewer's Original switch (the buttons are ViewerTools): the original image
    // in place of the preview
    let original = $derived(mode === 'view' && showOriginal && asset ? originalImage(asset) : undefined);

    // A Live Photo in the viewer: its motion plays over the photo once when it opens
    // (if autoplay is on) and again on hover, then the photo is back. With sound: the
    // click that opened the viewer allows it; if not, muted.
    let liveVideos = $derived(mode === 'view' && asset?.kind === 'live' ? videos : []);
    let liveDone = $state<string | null>(null);   // the item whose autoplay has ended
    let liveReplay = $state<string | null>(null); // the item hovered to play again
    let livePlaying = $derived(liveVideos.length > 0 && (liveReplay === item.guid
        || (viewerPrefs.autoplayLive && liveDone !== item.guid)));
    const playLive: Attachment<HTMLVideoElement> = (video) => {
        video.play().catch(() => {
            video.muted = true;
            video.play().catch(() => {});
        });
    };

    // The viewer: the image at its own pixel size, fitted into the screen — never
    // stretched. srcset makes an <img> as wide as `sizes` (100vw), so the box is sized
    // here from the image the asset has. Unknown size: the image's natural size.
    let shown = $derived(asset && mode === 'view' ? (original ?? biggestImage(asset)) : undefined);
    let fit = $derived(shown?.w && shown.h
        ? {width: `min(${shown.w}px, 100vw, calc(100vh * ${shown.w} / ${shown.h}))`, ratio: `${shown.w} / ${shown.h}`}
        : undefined);
</script>

<!-- The viewer plays a video; a Live Photo is a photo there (its motion is the tile's hover) -->
{#if asset && mode === 'view' && videos.length && asset.kind !== 'live'}
    {@const poster = fallbackImage(asset)}
    <video controls autoplay={viewerPrefs.autoplayVideo} playsinline poster={poster && assetUrl(poster)}
           onclick={(e) => e.stopPropagation()}>
        {#each videos as video (video.src)}
            <source src={video.src} type={video.type}>
        {/each}
    </video>
{:else if asset && hasStill}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="asset"
         class:view={mode === 'view'}
         class:natural={mode === 'view' && !fit}
         style:width={fit?.width}
         style:aspect-ratio={fit?.ratio}
         onmouseenter={() => { hovered = true; if (mode === 'view') liveReplay = item.guid; }}
         onmouseleave={() => (hovered = false)}>
        {#if original}
            <img class="original-image" src={assetUrl(original)} alt={item.guid}>
        {:else}
            <Picture {asset} {sizes} alt={item.guid} width={item.width} height={item.height}/>
        {/if}
        {#if mode === 'tile' && hovered && hasMotion}
            <div class="motion"><Motion {asset}/></div>
        {:else if mode === 'tile' && asset.kind}
            <!-- What moves is marked; the mark steps aside while it moves -->
            <KindBadge {asset}/>
        {/if}
        {#if livePlaying}
            <video class="live" {@attach playLive} playsinline
                   onended={() => { liveDone = item.guid; liveReplay = null; }}>
                {#each liveVideos as video (video.src)}
                    <source src={video.src} type={video.type}>
                {/each}
            </video>
        {/if}
    </div>
{:else if asset && mode === 'tile' && videos.length}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="asset"
         onmouseenter={() => (hovered = true)}
         onmouseleave={() => (hovered = false)}>
        <video class="tile-video" {@attach playOnHover} muted loop playsinline preload="metadata"
               onloadedmetadata={(e) => (fileDuration = e.currentTarget.duration)}>
            {#each videos as video (video.src)}
                <source src="{video.src}#t=0.1" type={video.type}>
            {/each}
        </video>
        {#if !hovered}
            <KindBadge {asset} duration={asset.duration || fileDuration}/>
        {/if}
    </div>
{:else if item.previewMime.startsWith("image")}
    <!-- From an older server (no asset): the default preview -->
    <Img item={item}/>
{:else if item.previewMime.startsWith("video")}
    <Video item={item}/>
{:else}
    <div>
        Can't render item {index}: {item.guid}
    </div>
{/if}

<style>
    .asset {
        position: relative;
        width: 100%;
        height: 100%;
    }

    .asset.view {
        height: auto;
    }

    .asset.view :global(img) {
        width: 100%;
        height: 100%;
        max-width: none;
        max-height: none;
    }

    .asset.natural :global(img) {
        width: auto;
        height: auto;
        max-width: 100vw;
        max-height: 100vh;
    }

    .tile-video {
        display: block;
        width: 100%;
        height: 100%;
        object-fit: cover;
    }

    .motion {
        position: absolute;
        inset: 0;
    }

    .original-image {
        display: block;
        width: 100%;
        height: 100%;
        object-fit: contain;
    }

    .live {
        position: absolute;
        inset: 0;
        width: 100%;
        height: 100%;
        object-fit: contain;
    }

    video {
        display: block;
        max-width: 100vw;
        max-height: 100vh;
    }
</style>
