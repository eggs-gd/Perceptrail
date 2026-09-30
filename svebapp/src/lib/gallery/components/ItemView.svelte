<script lang="ts">
    import {type Item} from '$lib/stores'

    import Img from "./Img.svelte";
    import Video from "./Video.svelte";
    import Picture from "./Picture.svelte";
    import Motion from "./Motion.svelte";
    import {assetUrl, fallbackImage, originalViewable, playableVideos} from "./asset";

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
    let hasStill = $derived(!!asset && (asset.edit.length > 0 || asset.stills.length > 0));
    let videos = $derived(asset ? playableVideos(asset) : []);
    let hasMotion = $derived(videos.length > 0 || (asset?.frames.length ?? 0) > 1);
    let hovered = $state(false);
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
        <Picture {asset} {sizes} alt={item.guid} width={item.width} height={item.height}/>
        {#if mode === 'tile' && hovered && hasMotion}
            <div class="motion"><Motion {asset}/></div>
        {/if}
        {#if mode === 'view' && asset.original}
            <!-- The browser opens what it can show (HEIC in Safari); the rest downloads -->
            <a class="original"
               href={assetUrl(asset.original)}
               target="_blank" rel="noopener"
               download={originalViewable(asset) ? undefined : ''}
               onclick={(e) => e.stopPropagation()}>Original</a>
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

    .motion {
        position: absolute;
        inset: 0;
    }

    .original {
        position: fixed;
        right: 1rem;
        bottom: 1rem;
        padding: 0.4rem 0.8rem;
        border-radius: 0.3rem;
        background: rgb(0 0 0 / 0.6);
        color: #fff;
        font: 0.9rem system-ui, sans-serif;
        text-decoration: none;
    }

    video {
        display: block;
        max-width: 100vw;
        max-height: 100vh;
    }
</style>
