<script lang="ts">
    import type {Item} from "$lib/gallery";
    export let item: Item;

</script>

<img src={item.src} width={item.width} height={item.height} alt={item.src}/>
