<script lang="ts">
    import type {Item} from "$lib/gallery";
    interface Props {
        item: Item;
    }

    let { item }: Props = $props();

</script>

<video src={item.src} width={item.width} height={item.height}>
    <track src={item.src} kind="captions">
</video>
