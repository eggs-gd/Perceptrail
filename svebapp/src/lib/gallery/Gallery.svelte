<script lang="ts">
    import Img from "./components/Img.svelte";
    import {layoutRaw} from "./layout";
    import type {Item, ItemScaled} from "./types";
    import {goto} from "$app/navigation";


    export let images: Item[] = [];
    export let rowHeight = 220;
    export let gutter = 8;
    export let imageComponent = Img;

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
        {#each scaledImages as {
            index,
            ratio,
            scaledHeight,
            scaledWidth,
            isLastInRow,
            isLastRow,
            scaledWidthPc,
            ...image
        }}
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <div class="image"
                 style={imgStyle(scaledWidth, scaledHeight, isLastInRow, isLastRow )}
                 on:click={(_) => goto("/item/" + index)}>
                <slot {index} {image}>
                    <svelte:component this={imageComponent} {...image}/>
                </slot>
            </div>
        {/each}
    </div>
</div>
