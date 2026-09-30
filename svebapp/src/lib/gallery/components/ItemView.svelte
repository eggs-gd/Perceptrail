<script lang="ts">
    import {type Item} from '$lib/stores'

    import Img from "./Img.svelte";
    import Video from "./Video.svelte";

    interface Props {
        item: Item;
        index: number;
    }

    let {item, index}: Props = $props();

</script>


<!-- The preview decides, not the original: a video may be shown as its photo -->
{#if item.previewMime.startsWith("image")}
    <Img item={item}/>
{:else if item.previewMime.startsWith("video")}
    <Video item={item}/>
{:else}
    <div>
        Can't render item:
        {item}
    </div>
{/if}
