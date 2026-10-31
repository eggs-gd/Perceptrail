<script lang="ts">
    import type {Item} from "#lib/stores/index.js";
    import {PUBLIC_API_PATH} from '$app/env/public'

    interface Props {
        item: Item;
    }

    let {item}: Props = $props();

</script>

<video src={`${PUBLIC_API_PATH}/assets/${item.guid}`} width={item.width} height={item.height} preload="metadata">
    <track src={`${PUBLIC_API_PATH}/assets/${item.guid}`} kind="captions">
</video>
