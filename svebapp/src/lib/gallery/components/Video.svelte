<script lang="ts">
    import type {Item} from "$lib/stores";
    import {PUBLIC_API_PATH} from '$env/static/public'

    interface Props {
        item: Item;
    }

    let {item}: Props = $props();

</script>

<video src={`${PUBLIC_API_PATH}/assets/${item.guid}`} width={item.width} height={item.height}>
    <track src={`${PUBLIC_API_PATH}/assets/${item.guid}`} kind="captions">
</video>
