<script lang="ts">
    import type {Asset} from '$lib/stores';
    import {assetUrl, fallbackImage, pictureSources} from './asset';

    interface Props {
        asset: Asset;
        /** How wide it is shown ("412px", "100vw"): the browser picks the size by it */
        sizes: string;
        alt: string;
        width?: number;
        height?: number;
        /**
         * eager: the gallery renders only a window of tiles (with a margin above and
         * below), so its images load at once — lazy would hold back exactly the
         * margin that is there to load them before they scroll in
         */
        loading?: 'eager' | 'lazy';
    }

    let {asset, sizes, alt, width, height, loading = 'eager'}: Props = $props();

    let sources = $derived(pictureSources(asset));
    let fallback = $derived(fallbackImage(asset));
</script>

<picture>
    {#each sources as source (source.type)}
        <source type={source.type} srcset={source.srcset} {sizes}>
    {/each}
    {#if fallback}
        <img src={assetUrl(fallback)} {width} {height} {alt} {loading} decoding="async">
    {/if}
</picture>

<style>
    picture {
        display: block;
        width: 100%;
        height: 100%;
    }

    img {
        display: block;
        width: 100%;
        height: 100%;
        object-fit: contain;
    }
</style>
