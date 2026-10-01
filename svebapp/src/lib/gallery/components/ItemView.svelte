<script lang="ts">
    import {type Item} from '$lib/stores'
    import type {Attachment} from 'svelte/attachments';

    import Img from "./Img.svelte";
    import Video from "./Video.svelte";
    import Picture from "./Picture.svelte";
    import Motion from "./Motion.svelte";
    import KindBadge from "./KindBadge.svelte";
    import {assetUrl, fallbackImage, hasImage, playableVideos} from "./asset";

    interface Props {
        item: Item;
        index: number;
        /** tile: the gallery (motion on hover); view: the viewer (full size, the original) */
        mode?: 'tile' | 'view';
        /** How wide it is shown: the browser picks the size of the image by it */
        sizes?: string;
    }

    let {item, index, mode = 'tile', sizes = '100vw'}: Props = $props();

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

    // The viewer's Original switch: the original in place of the preview. Only an
    // image is tried — the browser tells by loading it (HEIC: Safari yes, Chrome no);
    // what it cannot show (that HEIC, RAW, a video it cannot play) is downloaded.
    // Keyed by the item: the next item opens with its preview again.
    let originalFor = $state<string | null>(null);
    let failedFor = $state<string | null>(null);
    let showOriginal = $derived(originalFor === item.guid);
    let originalIsImage = $derived(!!asset?.original?.mime.startsWith('image/') && failedFor !== item.guid);
</script>

{#if asset && mode === 'view' && videos.length}
    {@const poster = fallbackImage(asset)}
    <video controls autoplay playsinline poster={poster && assetUrl(poster)}
           onclick={(e) => e.stopPropagation()}>
        {#each videos as video (video.src)}
            <source src={video.src} type={video.type}>
        {/each}
    </video>
{:else if asset && hasStill}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="asset"
         onmouseenter={() => (hovered = true)}
         onmouseleave={() => (hovered = false)}>
        {#if showOriginal && asset.original}
            <img class="original-image" src={assetUrl(asset.original)} alt={item.guid}
                 onerror={() => { failedFor = item.guid; originalFor = null; }}>
        {:else}
            <Picture {asset} {sizes} alt={item.guid} width={item.width} height={item.height}/>
        {/if}
        {#if mode === 'tile' && hovered && hasMotion}
            <div class="motion"><Motion {asset}/></div>
        {:else if mode === 'tile' && asset.kind}
            <!-- What moves is marked; the mark steps aside while it moves -->
            <KindBadge {asset}/>
        {/if}
        {#if mode === 'view' && asset.original}
            {#if originalIsImage}
                <button class="original" class:on={showOriginal}
                        onclick={(e) => { e.stopPropagation(); originalFor = showOriginal ? null : item.guid; }}>
                    Original
                </button>
            {:else}
                <a class="original" href={assetUrl(asset.original)} download
                   onclick={(e) => e.stopPropagation()}>Download original</a>
            {/if}
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

    .original {
        position: fixed;
        right: 1rem;
        bottom: 1rem;
        padding: 0.4rem 0.8rem;
        border-radius: 0.3rem;
        background: rgb(0 0 0 / 0.6);
        color: #fff;
        border: 1px solid transparent;
        font: 0.9rem system-ui, sans-serif;
        text-decoration: none;
        cursor: pointer;
    }

    .original.on {
        border-color: #fff;
        background: rgb(255 255 255 / 0.25);
    }

    video {
        display: block;
        max-width: 100vw;
        max-height: 100vh;
    }
</style>
