<script lang="ts">
    import {type Item} from '../types'

    import Img from "./Img.svelte";
    import Video from "./Video.svelte";

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
