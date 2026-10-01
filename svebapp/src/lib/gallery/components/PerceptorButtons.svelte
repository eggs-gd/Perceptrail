<script lang="ts">
    import {iconMask, perceptors} from '../perceptors.svelte';

    interface Props {
        /** A navigator was picked (the active one too: back to the sheet around the photo) */
        onpick: (name: string) => void;
    }

    let {onpick}: Props = $props();
</script>

<!-- One button per navigator: its icon (a mask — the button's colour paints it, an SVG
     from a plugin cannot run anything), its title and help as the tooltip -->
{#each perceptors.list as p (p.name)}
    <button class="tool" class:on={perceptors.active === p.name}
            title={p.help ? `${p.title}: ${p.help}` : p.title} aria-label={p.title}
            aria-pressed={perceptors.active === p.name}
            onclick={(e) => { e.stopPropagation(); onpick(p.name); }}>
        <span class="icon" style:mask-image={iconMask(p.icon)} style:-webkit-mask-image={iconMask(p.icon)}></span>
    </button>
{/each}

<style>
    .tool {
        display: flex;
        padding: 0.45rem;
        border: 1px solid transparent;
        border-radius: 50%;
        background: rgb(0 0 0 / 0.55);
        color: rgb(255 255 255 / 0.7);
        cursor: pointer;
    }

    .tool:hover {
        color: #fff;
    }

    .tool.on {
        border-color: rgb(255 255 255 / 0.8);
        color: #fff;
    }

    .icon {
        display: block;
        width: 20px;
        height: 20px;
        background: currentColor;
        mask-size: contain;
        mask-repeat: no-repeat;
        -webkit-mask-size: contain;
        -webkit-mask-repeat: no-repeat;
    }
</style>
