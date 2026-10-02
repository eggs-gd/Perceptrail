<script lang="ts">
    import type {Asset} from '$lib/stores';
    import type {Attachment} from 'svelte/attachments';
    import {assetUrl, hoverUrl, playableVideos} from './asset';

    interface Props {
        asset: Asset;
    }

    let {asset}: Props = $props();

    // Apple's video frames play as a flip-book when there is no video to play
    const FRAME_MS = 200;
    // Apple Photos on demand: the video is asked for only when the pointer stays —
    // passing over the sheet downloads nothing
    const ASK_AFTER_MS = 250;

    let videos = $derived(playableVideos(asset));
    // No video here yet: the server asks Photos for it (a video's 360p, a Live
    // Photo's motion); meanwhile the frames, if any. The video goes on top once it
    // plays — it wins over the frames.
    let hover = $derived(videos.length ? undefined : hoverUrl(asset));
    let flipbook = $derived(!videos.length && asset.frames.length > 1);
    let playing = $state(false);

    // The source is set only after the pointer stayed: that is the request
    const askLater: Attachment<HTMLVideoElement> = (video) => {
        const timer = setTimeout(() => {
            video.src = hover!;
            video.play().catch(() => {});
        }, ASK_AFTER_MS);
        return () => clearTimeout(timer);
    };

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
{:else}
    {#if flipbook}
        <img src={assetUrl(asset.frames[0])} alt="" {@attach flip}>
    {/if}
    {#if hover}
        <video class="on-demand" class:playing muted loop playsinline {@attach askLater}
               onplaying={() => (playing = true)}></video>
    {/if}
{/if}

<style>
    video, img {
        display: block;
        width: 100%;
        height: 100%;
        object-fit: cover;
    }

    /* Over the frames (or the tile), shown once it plays */
    .on-demand {
        position: absolute;
        inset: 0;
        opacity: 0;
    }

    .on-demand.playing {
        opacity: 1;
    }
</style>
