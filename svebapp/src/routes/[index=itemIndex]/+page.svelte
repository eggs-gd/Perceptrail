<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import {goto} from "$app/navigation";
    import {page} from '$app/stores';
    import {currentIndex, currentItem} from "../stores";

    const id: number = Number($page.params.index);

</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="image"
onclick={() => goto("/")}>

    <ItemView item={$currentItem} index={$currentIndex}/>

</div>

<style>

    .image {
        position: relative;
        height: 100%;
    }

    .image > :global(*) {
        width: 100%;
        height: 100%;
    }

</style>