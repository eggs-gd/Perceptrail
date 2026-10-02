<script lang="ts">
    import type {Asset} from '$lib/stores';
    import type {Attachment} from 'svelte/attachments';
    import {assetUrl, hoverUrl, playableVideos} from './asset';

    interface Props {
        asset: Asset;
        /** The video is on its way (the tile's badge spins meanwhile) */
        loading?: boolean;
    }

    let {asset, loading = $bindable(false)}: Props = $props();

    // Apple's video frames play as a flip-book when there is no video to play
    const FRAME_MS = 200;
    // Apple Photos on demand: the video is asked for only when the pointer stays —
    // passing over the sheet downloads nothing
    const ASK_AFTER_MS = 250;

    let videos = $derived(playableVideos(asset));
    // No video here yet: the server asks Photos for it (a video's 360p, a Live
    // Photo's motion); meanwhile the frames, if any. The video fades in over them
    // once it plays — it wins over the frames.
    let hover = $derived(videos.length ? undefined : hoverUrl(asset));
    let flipbook = $derived(!videos.length && asset.frames.length > 1);
    let playing = $state(false);

    // A video here: loading until it plays
    const loadingUntilPlays: Attachment<HTMLVideoElement> = () => {
        loading = true;
        return () => (loading = false);
    };

    // On demand: the source is set only after the pointer stayed — that is the request
    const askLater: Attachment<HTMLVideoElement> = (video) => {
        const timer = setTimeout(() => {
            loading = true;
            video.src = hover!;
            video.play().catch(() => {});
        }, ASK_AFTER_MS);
        return () => {
            clearTimeout(timer);
            loading = false;
        };
    };

    const played = () => {
        playing = true;
        loading = false;
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
    <video class="fades" class:playing autoplay muted loop playsinline {@attach loadingUntilPlays}
           onplaying={played} onerror={() => (loading = false)}>
        {#each videos as video (video.src)}
            <source src={video.src} type={video.type}>
        {/each}
    </video>
{:else}
    {#if flipbook}
        <img class="frames" src={assetUrl(asset.frames[0])} alt="" {@attach flip}>
    {/if}
    {#if hover}
        <video class="fades" class:playing muted loop playsinline {@attach askLater}
               onplaying={played} onerror={() => (loading = false)}></video>
    {/if}
{/if}

<style>
    video, img {
        position: absolute;
        inset: 0;
        display: block;
        width: 100%;
        height: 100%;
        object-fit: cover;
    }

    /* Over the frames (or the tile's still), faded in once it plays */
    .fades {
        opacity: 0;
        transition: opacity 200ms;
    }

    .fades.playing {
        opacity: 1;
    }

    .frames {
        animation: fade-in 150ms;
    }

    @keyframes fade-in {
        from {
            opacity: 0;
        }
    }
</style>
