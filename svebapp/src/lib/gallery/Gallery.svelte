<script lang="ts">
    import {flip} from 'svelte/animate';
    import { fade } from 'svelte/transition';
    import ItemView from "./components/ItemView.svelte";
    import {layoutRaw} from "./layout";
    import {type Item} from "./types";

    interface Props {
        images?: Item[];
        rowHeight?: number;
        gutter?: number;
        selectItem: (item: number) => void,
        openItem: (item: number) => void,
    }

    let {
        images = [],
        rowHeight = 220,
        gutter = 8,
        selectItem,
        openItem
    }: Props = $props();

    let width = $state(1280);
    let scaledImages = $derived(layoutRaw({
            images,
            containerWidth: width,
            targetHeight: rowHeight,
            gutter
        })
    );

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

</script>

<div class="masonry" bind:clientWidth={width}>
    <div class="container" style="width: {width}px" class:hidden={!width}>
        {#each scaledImages as itm, index (itm.guid)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="image"
                 transition:fade={{ duration: 2000 }}
                 animate:flip="{{ duration: 2000 }}"
                 style={imgStyle(itm.scaledWidth, itm.scaledHeight, itm.isLastInRow, itm.isLastRow )}
                 onclick={() => {
                     selectItem(index);
                     openItem(index);
                 }}>
                <ItemView item={itm} index={index}/>
            </div>
        {/each}
    </div>
</div>

<style>
    .masonry {
        max-width: 100%;
    }

    .container {
        display: flex;
        flex-wrap: wrap;
    }

    .image {
        /*transition: transform 0.5s ease;*/
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
