<script lang="ts">
    import {updateLayout} from "$lib/workers";
    import {rowHeight, screenWidth} from "$lib/stores";

    let {data, children} = $props();

    $effect(() => updateLayout($screenWidth, $rowHeight))
</script>
<main>
    <!-- +page.svelte is `@render`ed here -->
    {@render children()}
</main>
