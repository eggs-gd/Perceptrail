<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import {items} from "../stores.js";
    import {page} from '$app/stores';

    const id: number = Number($page.params.index);
</script>

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

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="image">

    <ItemView item={$items[id]} index={id}/>

</div>
