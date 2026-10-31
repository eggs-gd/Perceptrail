<script lang="ts">
    import PerceptorButtons from './PerceptorButtons.svelte';
    import GalleryMenu from './GalleryMenu.svelte';

    interface Props {
        /** A view was picked (the gallery keeps the photo in the middle in place) */
        onpick: (slug: string) => void;
    }

    let {onpick}: Props = $props();

    // The gallery's toolbar: the views' buttons and the menu (the side panel's pin,
    // which views have a button)
</script>

<div class="tools" role="toolbar">
    <PerceptorButtons {onpick}/>
    <GalleryMenu/>
</div>

<style>
    .tools {
        position: fixed;
        top: 1rem;
        right: 1rem;
        z-index: 2;
        display: flex;
        gap: 0.5rem;
    }
</style>
