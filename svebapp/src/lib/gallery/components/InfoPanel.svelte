<script lang="ts">
    import {PUBLIC_API_PATH} from '$env/static/public';
    import type {Asset} from '$lib/stores';
    import {iconMask} from '../perceptors.svelte';
    import {fullHere} from './asset';

    interface Props {
        guid: string;
        asset?: Asset;
        onclose: () => void;
    }

    let {guid, asset, onclose}: Props = $props();

    // The viewer's info panel: what each perceptor knows about the photo
    // (GET /items/:guid/info) and every file of its group — the original, edits,
    // derivatives, motion, frames, sidecars — each to download (GET /items/:guid/files)

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

    interface GroupFile {
        name: string;
        role: string;
        mime: string;
        size: number;
        w?: number;
        h?: number;
        url: string;
    }

    let files = $derived.by(() => {
        const g = guid;
        return fetch(`${PUBLIC_API_PATH}/items/${encodeURIComponent(g)}/files`)
            .then((r) => (r.ok ? r.json() as Promise<GroupFile[]> : []));
    });

    const ROLES: Record<string, string> = {
        original: 'Original', edit: 'Edit', still: 'Preview', motion: 'Motion', frames: 'Frame', meta: 'Sidecar',
    };
    const ORDER = ['original', 'edit', 'still', 'motion', 'frames', 'meta'];
    const byRole = (fs: GroupFile[]) =>
        [...fs].sort((a, b) => (ORDER.indexOf(a.role) + 1 || 99) - (ORDER.indexOf(b.role) + 1 || 99));
    const type = (f: GroupFile) => (f.mime.split('/')[1] ?? f.mime).replace(/^x-|^vnd\./, '').toUpperCase();
    const bytes = (n: number) => n >= 1e6 ? `${(n / 1e6).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1e3))} KB`;
    const details = (f: GroupFile) =>
        [type(f), f.w && f.h ? `${f.w} × ${f.h}` : '', bytes(f.size)].filter(Boolean).join(' · ');
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
    <section>
        <h3>Files</h3>
        {#if asset}
            <p class="note">Full resolution {fullHere(asset) ? 'here' : 'in iCloud only'}</p>
        {/if}
        {#await files then list}
            <ul class="files">
                {#each byRole(list) as f (f.url)}
                    <li>
                        <span class="file">
                            <span class="role">{ROLES[f.role] ?? 'Other'}</span>
                            <span class="name" title={f.name}>{f.name}</span>
                            <span class="details">{details(f)}</span>
                        </span>
                        <a class="download" href={`${PUBLIC_API_PATH}${f.url}?download=1`}
                           title="Download {f.name}" aria-label="Download {f.name}">
                            <svg viewBox="0 0 24 24" aria-hidden="true">
                                <path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 19.5h14" fill="none" stroke="currentColor"
                                      stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                            </svg>
                        </a>
                    </li>
                {/each}
            </ul>
        {/await}
    </section>
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

    header button:hover {
        color: #fff;
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

    .note {
        margin: 0 1rem 0.4rem;
        color: #aaa;
    }

    .files {
        margin: 0;
        padding: 0 0.6rem 0 1rem;
        list-style: none;
    }

    .files li {
        display: flex;
        align-items: center;
        gap: 0.4rem;
        padding: 0.25rem 0;
        border-top: 1px solid rgb(255 255 255 / 0.06);
    }

    .file {
        display: grid;
        flex: 1;
        min-width: 0;
    }

    .role {
        color: #aaa;
        font-size: 0.75rem;
    }

    .name {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .details {
        color: #aaa;
        font-size: 0.75rem;
    }

    .download {
        display: flex;
        padding: 0.3rem;
        border-radius: 50%;
        color: #ccc;
    }

    .download:hover {
        color: #fff;
        background: rgb(255 255 255 / 0.1);
    }

    .download svg {
        width: 18px;
        height: 18px;
    }
</style>
