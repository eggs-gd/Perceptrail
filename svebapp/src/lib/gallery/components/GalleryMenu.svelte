<script lang="ts">
    import {iconMask, perceptors, toggleHidden, togglePinned} from '../perceptors.svelte';

    // The gallery's menu (a burger): the side panel's pin and which views have a
    // button. A hidden view still works by its URL.
    let open = $state(false);

    function close(e: Event) {
        if (open && !(e.target as Element).closest?.('.menu')) open = false;
    }
</script>

<svelte:window onclick={close} onkeydown={(e) => { if (e.key === 'Escape') open = false; }}/>

<div class="menu">
    <button class="tool" class:on={open} title="Menu" aria-label="Menu" aria-expanded={open}
            onclick={() => (open = !open)}>
        <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M4 7h16M4 12h16M4 17h16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
        </svg>
    </button>
    {#if open}
        <div class="dropdown" role="menu">
            <label class="row">
                <input type="checkbox" checked={perceptors.pinned} onchange={togglePinned}>
                <span>Pin the side panel</span>
            </label>
            {#if perceptors.list.length}
                <div class="heading">Views</div>
                {#each perceptors.list as p (p.slug)}
                    <label class="row" title={p.help}>
                        <input type="checkbox" checked={!perceptors.hidden.includes(p.slug)}
                               onchange={() => toggleHidden(p.slug)}>
                        <span class="icon" style:mask-image={iconMask(p.icon)} style:-webkit-mask-image={iconMask(p.icon)}></span>
                        <span>{p.title}</span>
                    </label>
                {/each}
            {/if}
        </div>
    {/if}
</div>

<style>
    .menu {
        position: relative;
    }

    .tool {
        display: flex;
        padding: 0.45rem;
        border: 1px solid transparent;
        border-radius: 50%;
        background: rgb(0 0 0 / 0.55);
        color: rgb(255 255 255 / 0.7);
        cursor: pointer;
    }

    .tool:hover,
    .tool.on {
        color: #fff;
    }

    svg {
        width: 20px;
        height: 20px;
    }

    .dropdown {
        position: absolute;
        top: calc(100% + 0.5rem);
        right: 0;
        min-width: 13rem;
        padding: 0.4rem 0;
        border-radius: 0.6rem;
        background: rgb(20 20 20 / 0.95);
        box-shadow: 0 6px 24px rgb(0 0 0 / 0.5);
        color: #eee;
        font: 0.9rem system-ui, sans-serif;
    }

    .heading {
        margin: 0.4rem 0.9rem 0.2rem;
        color: rgb(255 255 255 / 0.5);
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .row {
        display: flex;
        align-items: center;
        gap: 0.6rem;
        padding: 0.4rem 0.9rem;
        cursor: pointer;
    }

    .row:hover {
        background: rgb(255 255 255 / 0.08);
    }

    .icon {
        display: block;
        width: 18px;
        height: 18px;
        background: currentColor;
        mask-size: contain;
        mask-repeat: no-repeat;
        -webkit-mask-size: contain;
        -webkit-mask-repeat: no-repeat;
    }
</style>
