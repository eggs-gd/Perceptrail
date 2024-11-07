<script lang="ts">
    let { data, children } = $props();
</script>
<main>
    <!-- +page.svelte is `@render`ed here -->
    {@render children()}
</main>
