<script lang="ts">
    import Img from "./Img.svelte";
    import {type Item} from '../types'

    interface Props {
        item: Item;
        index: number;
    }

    let {item, index}: Props = $props();

</script>

<Img item={item} />
