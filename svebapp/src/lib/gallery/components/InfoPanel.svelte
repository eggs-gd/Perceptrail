<script lang="ts">
    import {PUBLIC_API_PATH} from '$env/static/public';
    import type {Asset, Rendition} from '$lib/stores';
    import {iconMask} from '../perceptors.svelte';
    import {toggleViewerPref, viewerPrefs} from './viewerPrefs.svelte';

    interface Props {
        guid: string;
        asset?: Asset;
        onclose: () => void;
    }

    let {guid, asset, onclose}: Props = $props();

    // The viewer's info panel: what each perceptor knows about the photo
    // (GET /items/:guid/info) and the asset's files (the client has them already)

    interface Info {
        slug: string;
        title: string;
        icon: string;
        facts: {label: string, value: string}[];
    }

    let info = $derived.by(() => {
        const g = guid;
        return fetch(`${PUBLIC_API_PATH}/items/${encodeURIComponent(g)}/info`)
            .then((r) => (r.ok ? r.json() as Promise<Info[]> : []));
    });

    const kind = (r: Rendition) => r.mime.replace(/^(image|video)\//, '').toUpperCase();
    const dims = (r: Rendition) => (r.w && r.h ? ` · ${r.w} × ${r.h}` : '');

    // The files: the source (or that it is only in iCloud), the edit, previews, motion
    let files = $derived.by(() => {
        if (!asset) return [];
        const out: {label: string, value: string}[] = [];
        out.push(asset.original
            ? {label: 'Original', value: kind(asset.original) + dims(asset.original)}
            : {label: 'Original', value: 'in iCloud only'});
        if (asset.edit.length) out.push({label: 'Edited', value: asset.edit.map(kind).join(', ')});
        if (asset.stills.length) {
            const biggest = [...asset.stills].sort((a, b) => (b.w ?? 0) - (a.w ?? 0))[0];
            out.push({label: 'Previews', value: `${asset.stills.length}, up to ${biggest.w ?? '?'} px`});
        }
        if (asset.motion.length) out.push({label: 'Motion', value: asset.motion.map(kind).join(', ')});
        if (asset.frames.length) out.push({label: 'Frames', value: String(asset.frames.length)});
        return out;
    });
</script>

{#snippet block(title: string, icon: string | undefined, facts: {label: string, value: string}[])}
    <section>
        <h3>
            {#if icon}<span class="icon" style:mask-image={iconMask(icon)} style:-webkit-mask-image={iconMask(icon)}></span>{/if}
            {title}
        </h3>
        <dl>
            {#each facts as f (f.label)}
                <dt>{f.label}</dt>
                <dd>{f.value}</dd>
            {/each}
        </dl>
    </section>
{/snippet}

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<aside class="panel" onclick={(e) => e.stopPropagation()}>
    <header>
        <span>Info</span>
        <button class:on={viewerPrefs.infoPinned} aria-pressed={viewerPrefs.infoPinned}
                title={viewerPrefs.infoPinned ? 'Stays open' : 'Keep it open'}
                aria-label="Keep the panel open" onclick={() => toggleViewerPref('infoPinned')}>
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M9 4h6l-1 6 3 3H7l3-3zM12 13v7" fill="none" stroke="currentColor" stroke-width="1.6"
                      stroke-linejoin="round" stroke-linecap="round"/>
            </svg>
        </button>
        <button title="Close" aria-label="Close the panel" onclick={onclose}>
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
            </svg>
        </button>
    </header>
    {#await info then perceptors}
        {#each perceptors as p (p.slug)}
            {@render block(p.title, p.icon, p.facts)}
        {/each}
    {/await}
    {#if files.length}
        {@render block('Files', undefined, files)}
    {/if}
</aside>

<style>
    .panel {
        position: fixed;
        top: 4.5rem;
        right: 1rem;
        bottom: 1rem;
        z-index: 1;
        width: 19rem;
        overflow-y: auto;
        padding: 0.4rem 0 0.8rem;
        border-radius: 0.8rem;
        background: rgb(20 20 20 / 0.92);
        color: #eee;
        font: 0.85rem system-ui, sans-serif;
        /* The layout turns pointer events off under the viewer (main.viewing) */
        pointer-events: auto;
    }

    header {
        display: flex;
        align-items: center;
        gap: 0.3rem;
        padding: 0.3rem 0.6rem 0.3rem 1rem;
        font-weight: 600;
    }

    header span {
        flex: 1;
    }

    header button {
        display: flex;
        padding: 0.3rem;
        border: 1px solid transparent;
        border-radius: 50%;
        background: none;
        color: rgb(255 255 255 / 0.6);
        cursor: pointer;
    }

    header button:hover,
    header button.on {
        color: #fff;
    }

    header button.on {
        border-color: rgb(255 255 255 / 0.6);
    }

    header svg {
        width: 18px;
        height: 18px;
    }

    section {
        padding: 0.5rem 1rem;
        border-top: 1px solid rgb(255 255 255 / 0.08);
    }

    h3 {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin: 0 0 0.4rem;
        font-size: 0.8rem;
        font-weight: 600;
        color: rgb(255 255 255 / 0.7);
    }

    .icon {
        display: block;
        width: 16px;
        height: 16px;
        background: currentColor;
        mask-size: contain;
        mask-repeat: no-repeat;
        -webkit-mask-size: contain;
        -webkit-mask-repeat: no-repeat;
    }

    dl {
        display: grid;
        grid-template-columns: max-content 1fr;
        gap: 0.25rem 0.8rem;
        margin: 0;
    }

    dt {
        color: rgb(255 255 255 / 0.5);
    }

    dd {
        margin: 0;
        overflow-wrap: anywhere;
    }
</style>
