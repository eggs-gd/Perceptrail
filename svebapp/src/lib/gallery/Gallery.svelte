<script lang="ts">
    import {flip} from 'svelte/animate';
    import {fade} from 'svelte/transition';
    import ItemView from "./components/ItemView.svelte";
    import type {LayoutItem} from "$lib/stores";
    import {screenWidth} from "$lib/stores";


    interface Props {
        images: LayoutItem[];
        gutter?: number;
        selectItem: (item: number) => void,
        openItem: (item: number) => void,
    }

    let {
        images = [],
        gutter = 8,
        selectItem,
        openItem
    }: Props = $props();

    function imgStyle(width: number, height: number, scale: number) {
        let margin = gutter * 0.5 + 'px';
        let flex = `0 0 ${width * scale - gutter * 0.5}px`;

        return `height: ${height * scale - gutter * 0.5}px; flex: ${flex}; margin: ${margin}; border: 1px solid green;`;
    }


</script>

<div class="masonry" bind:clientWidth={$screenWidth}>
    <div class="container" style="width: {$screenWidth}px" class:hidden={!$screenWidth}>
        {#each images as itm, index (itm.guid)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="image"

                 transition:fade={{ duration: 2000 }}
                 animate:flip="{{ duration: 2000 }}"
                 style={imgStyle(itm.width, itm.height, itm.scale)}
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
