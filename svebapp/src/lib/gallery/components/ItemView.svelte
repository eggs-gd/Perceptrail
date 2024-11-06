<script lang="ts">
    import Img from "./Img.svelte";
    import {type Item} from '../types'
    import Video from "$lib/gallery/components/Video.svelte";

    interface Props {
        item: Item;
        index: number;
    }

    let {item, index}: Props = $props();

</script>

{#if item.mimeType.startsWith("image")}
    <Img item={item}/>
{:else if item.mimeType.startsWith("video")}
    <Video item={item}/>
{:else}
    <div>
        Can't render item:
        {item}
    </div>
{/if}
