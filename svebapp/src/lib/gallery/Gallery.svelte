<script lang="ts">
    import ItemView from "./components/ItemView.svelte";
    import {layoutRaw} from "./layout";
    import {GalleryEvent, type Item, type ItemScaled} from "./types";
    import { createEventDispatcher } from 'svelte';

    export let images: Item[] = [];
    export let rowHeight = 220;
    export let gutter = 8;

    let scaledImages: ItemScaled[] = [];
    let width = 0;


    function imgStyle(scaledWidth: number, scaledHeight: number, isLastInRow: boolean, isLastRow: boolean) {
        let marginRight = gutter + 'px',
            flex = `0 0 ${scaledWidth}px`,
            marginBottom = isLastRow ? '0' : marginRight;

        if (isLastInRow) {
            marginRight = '0';
            flex = `1 1 ${scaledWidth - 4}px`;
        }

        return `height: ${scaledHeight}px; flex: ${flex}; margin-right: ${marginRight}; margin-bottom: ${marginBottom};`;
    }

    $: scaledImages = layoutRaw({
            images,
            containerWidth: width || 1280,
            targetHeight: rowHeight,
            gutter
        }
    );

    const dispatch = createEventDispatcher();

</script>

<style>
    .masonry {
        max-width: 100%;
    }

    .container {
        display: flex;
        flex-wrap: wrap;
    }

    .image {
        position: relative;
        height: 100%;
    }

    .image > :global(*) {
        width: 100%;
        height: 100%;
    }

    .hidden {
        visibility: hidden;
    }

</style>

<div class="masonry" bind:clientWidth={width}>
    <div class="container" style="width: {width}px" class:hidden={!width}>
        {#each scaledImages as itm, index}
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <div class="image"
                 style={imgStyle(itm.scaledWidth, itm.scaledHeight, itm.isLastInRow, itm.isLastRow )}
                 on:click={() => {
                     dispatch(GalleryEvent.selectItem, index);
                     dispatch(GalleryEvent.openItem, index);
                 }}>
                <slot {index} {itm}>
                    <svelte:component this={ItemView} item ={itm}/>
                </slot>
            </div>
        {/each}
    </div>
</div>
