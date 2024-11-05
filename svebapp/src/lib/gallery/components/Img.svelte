<script lang="ts">
    import type {Item} from "$lib/gallery";
    interface Props {
        item: Item;
    }

    let { item }: Props = $props();

</script>

<img src={item.src} width={item.width} height={item.height} alt={item.src}/>
