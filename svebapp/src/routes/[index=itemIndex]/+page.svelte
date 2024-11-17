<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import {goto} from "$app/navigation";
    import {page} from '$app/stores';
    import {items, currentIndex, currentItem} from "$lib/stores/stores";

    $currentIndex = Number($page.params.index);

    $currentItem = $items[$currentIndex]

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
