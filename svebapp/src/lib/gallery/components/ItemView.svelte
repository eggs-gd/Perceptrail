<script lang="ts">
    import Img from "./Img.svelte";
    import {type Item} from '../types'

    export let item: Item;

</script>

<Img item={item}/>
