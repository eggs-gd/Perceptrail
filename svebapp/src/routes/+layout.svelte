<script lang="ts">
    import {updateLayout} from "$lib/stores/proxy";
    import {rowHeight, screenWidth} from "$lib/stores/stores";

    let {data, children} = $props();

    $effect(() => updateLayout($screenWidth, $rowHeight))
</script>
<main>
    <!-- +page.svelte is `@render`ed here -->
    {@render children()}
</main>
