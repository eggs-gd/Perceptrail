<script lang="ts">
    import {type Item} from '$lib/stores'
    import type {Attachment} from 'svelte/attachments';

    import Img from "./Img.svelte";
    import Video from "./Video.svelte";
    import Picture from "./Picture.svelte";
    import Motion from "./Motion.svelte";
    import KindBadge from "./KindBadge.svelte";
    import {assetUrl, biggestImage, fallbackImage, hasImage, hoverUrl, mediumUrl, originalImage, playableVideos} from "./asset";
    import {viewerPrefs} from "./viewerPrefs.svelte";
    import {debug} from "$lib/app.svelte";

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
    // Apple Photos: a moving tile may ask for its video on hover (Motion)
    let hasMotion = $derived(videos.length > 0 || (asset?.frames.length ?? 0) > 1 || !!asset?.onDemand?.hover);
    let hovered = $state(false);
    // The hover's video is on its way: the badge spins
    let motionLoading = $state(false);

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
    // Apple Photos: no motion here yet — the server asks Photos for it
    let liveVideos = $derived.by(() => {
        if (mode !== 'view' || asset?.kind !== 'live') return [];
        if (videos.length) return videos;
        const hover = hoverUrl(asset);
        return hover ? [{src: hover, type: ''}] : [];
    });
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

    // Apple Photos: the viewer's medium rendition (the image, a video's 720p), asked
    // for when it opens — the image goes over what the asset has once it loads
    let medium = $derived(asset && mode === 'view' && !original ? mediumUrl(asset) : undefined);
    let mediumLoad = $state<{url: string, w?: number, h?: number}>();
    let better = $derived(medium && mediumLoad?.url === medium && mediumLoad.w ? mediumLoad : undefined);
    // Asked for and not answered yet (a failure answers too)
    let waiting = $derived(!!medium && mediumLoad?.url !== medium);

    // The viewer: the image at its own pixel size, fitted into the screen — stretched
    // only if the user switched that on. srcset makes an <img> as wide as `sizes`
    // (100vw), so the box is sized here from the image the asset has (the medium
    // once it came; while it comes, as big as the screen allows: it will be).
    // Unknown size: the image's natural size.
    let shown = $derived(asset && mode === 'view' ? (original ?? better ?? biggestImage(asset)) : undefined);
    let fit = $derived(shown?.w && shown.h
        ? {
            width: `min(${viewerPrefs.stretchSmall || waiting ? '' : `${shown.w}px, `}100vw, calc(100vh * ${shown.w} / ${shown.h}))`,
            ratio: `${shown.w} / ${shown.h}`,
        }
        : undefined);
</script>

<!-- The viewer plays a video; a Live Photo is a photo there (its motion is the tile's hover).
     Apple Photos: the 720p asked for first, what is here after it (a failed source
     falls through to the next) -->
{#if asset && mode === 'view' && asset.kind !== 'live' && (videos.length || (asset.kind === 'video' && medium))}
    {@const poster = fallbackImage(asset)}
    <video controls autoplay={viewerPrefs.autoplayVideo} playsinline poster={poster && assetUrl(poster)}
           onclick={(e) => e.stopPropagation()}>
        {#if medium}
            <source src={medium}>
        {/if}
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
            {#if medium}
                <img class="medium" class:ready={better} src={medium} alt=""
                     onload={(e) => {
                         const img = e.currentTarget as HTMLImageElement;
                         mediumLoad = {url: medium!, w: img.naturalWidth, h: img.naturalHeight};
                     }}
                     onerror={() => (mediumLoad = {url: medium!})}>
            {/if}
        {/if}
        {#if mode === 'tile' && hovered && hasMotion}
            <div class="motion"><Motion {asset} bind:loading={motionLoading}/></div>
        {/if}
        {#if mode === 'tile' && asset.kind}
            <!-- What moves is marked, also while it moves; it spins while the video comes -->
            <KindBadge {asset} loading={hovered && motionLoading}/>
        {/if}
        {#if livePlaying}
            <video class="live" {@attach playLive} playsinline
                   onended={() => { liveDone = item.guid; liveReplay = null; }}>
                {#each liveVideos as video (video.src)}
                    <source src={video.src} type={video.type || undefined}>
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
        <KindBadge {asset} duration={asset.duration || fileDuration}/>
    </div>
{:else if item.previewMime.startsWith("image")}
    <!-- From an older server (no asset): the default preview -->
    <Img item={item}/>
{:else if item.previewMime.startsWith("video")}
    <Video item={item}/>
{:else if debug()}
    <div>
        Can't render item {index}: {item.guid}
    </div>
{:else}
    <!-- Nothing the browser can show yet (the transcode comes later) -->
    <div class="nothing"></div>
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

    .nothing {
        width: 100%;
        height: 100%;
        background: rgb(255 255 255 / 0.04);
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

    /* Apple Photos' medium rendition, over the asset's own image once it loaded */
    .medium {
        position: absolute;
        inset: 0;
        object-fit: contain;
        opacity: 0;
        transition: opacity 150ms;
    }

    .medium.ready {
        opacity: 1;
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
