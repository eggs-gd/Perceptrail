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

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="image">

    <ItemView item={$items[id]}/>

</div>
