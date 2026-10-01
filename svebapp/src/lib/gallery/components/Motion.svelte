<script lang="ts">
    import type {Asset} from '$lib/stores';
    import type {Attachment} from 'svelte/attachments';
    import {assetUrl, playableVideos} from './asset';

    interface Props {
        asset: Asset;
    }

    let {asset}: Props = $props();

    // Apple's video frames play as a flip-book when there is no video to play
    const FRAME_MS = 200;

    let videos = $derived(playableVideos(asset));
    let flipbook = $derived(!videos.length && asset.frames.length > 1);

    // Turns the pages on the element itself: no component state per frame
    const flip: Attachment<HTMLImageElement> = (img) => {
        let frame = 0;
        const timer = setInterval(() => {
            frame = (frame + 1) % asset.frames.length;
            img.src = assetUrl(asset.frames[frame]);
        }, FRAME_MS);
        return () => clearInterval(timer);
    };
</script>

{#if videos.length}
    <video autoplay muted loop playsinline>
        {#each videos as video (video.src)}
            <source src={video.src} type={video.type}>
        {/each}
    </video>
{:else if flipbook}
    <img src={assetUrl(asset.frames[0])} alt="" {@attach flip}>
{/if}

<style>
    video, img {
        display: block;
        width: 100%;
        height: 100%;
        object-fit: cover;
    }
</style>
