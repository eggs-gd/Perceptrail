<script lang="ts">
    import ItemView from "$lib/gallery/components/ItemView.svelte";
    import {db} from '../stores';
    import {page} from '$app/stores';
    import {liveQuery} from "dexie";

    const id: number = Number($page.params.index);

    let items = liveQuery(
        () => db.items.toArray()
    );

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
