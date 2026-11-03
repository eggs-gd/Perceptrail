<script lang="ts">
    import {channel, commands, installs, origin, repo, version, type Channel} from '#lib/site.js';

    // What the page is, written for the search snippet (≤ 155 characters)
    const title = 'Perceptrail — a self-hosted photo gallery with points of view';
    const description = 'One endless sheet of your photos, ordered by date, place or size. Switch the view on any photo and stay on it. Self-hosted, reads your library only.';
    const here = installs[channel];
    const url = origin + here.path;
    const image = url + 'og.png';

    // The install shown: this build's channel first, the other one a click away
    let shown = $state<Channel>(channel);
    let lines = $derived(commands(installs[shown]));
    let copied = $state(false);

    async function copy() {
        await navigator.clipboard.writeText(lines.join('\n'));
        copied = true;
        setTimeout(() => (copied = false), 1500);
    }

    const structured = JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'SoftwareApplication',
        name: 'Perceptrail',
        description,
        url,
        applicationCategory: 'MultimediaApplication',
        operatingSystem: 'Linux, macOS (Docker)',
        softwareVersion: version,
        license: 'https://www.mozilla.org/en-US/MPL/2.0/',
        inLanguage: 'en',
        offers: {'@type': 'Offer', price: '0', priceCurrency: 'USD'},
        sameAs: [repo],
        publisher: {'@type': 'Organization', name: 'earwiG Games', url: 'https://eggs.gd'},
    });
</script>

<svelte:head>
    <title>{title}</title>
    <meta name="description" content={description}>
    <link rel="canonical" href={url}>
    {#if channel === 'develop'}
        <!-- What is being tested: the stable page is the one to find -->
        <meta name="robots" content="noindex">
    {/if}
    <meta property="og:type" content="website">
    <meta property="og:site_name" content="Perceptrail">
    <meta property="og:title" content={title}>
    <meta property="og:description" content={description}>
    <meta property="og:url" content={url}>
    <meta property="og:image" content={image}>
    <meta property="og:image:width" content="1200">
    <meta property="og:image:height" content="630">
    <meta property="og:image:alt" content="Perceptrail: one endless sheet of photos, seen by date, place or size">
    <meta property="og:locale" content="en">
    <meta name="twitter:card" content="summary_large_image">
    <meta name="twitter:title" content={title}>
    <meta name="twitter:description" content={description}>
    <meta name="twitter:image" content={image}>
    <!-- Structured data: our own constants at build time, no input of anyone's -->
    {@html `<script type="application/ld+json">${structured}<` + `/script>`}
</svelte:head>

<main>
    <header class="hero">
        <p class="kicker">Self-hosted photo gallery · {channel === 'develop' ? 'develop' : 'v' + version}</p>
        <h1>Perceptrail</h1>
        <p class="lead">
            One endless sheet of your photos, and you choose the point of view: by date, by place,
            by size. Switch the view on any photo — and stay on that photo.
        </p>
        <p class="actions">
            <a class="button primary" href="#install" data-cta="install" data-cta-placement="hero">Install</a>
            <a class="button" href={repo} data-cta="github" data-cta-placement="hero">GitHub</a>
        </p>
    </header>

    <section>
        <h2>Perceptors: points of view</h2>
        <p>
            A <em>perceptor</em> is a plugin that knows one thing about a photo and orders the sheet by it,
            with its own sections. The gallery has a button for each; the photo you are looking at stays
            in place while everything around it is ordered anew.
        </p>
        <ul class="grid">
            <li><strong>Date</strong> — the timeline, by year and month, in the zone the photo was taken in.</li>
            <li><strong>Places</strong> — from GPS: photos taken near each other side by side, sections by region and city.</li>
            <li><strong>Size</strong> and <strong>duration</strong> — by pixels and by length: the big shots, the long videos.</li>
            <li><strong>Coming</strong> — faces, colour, objects, and the <em>trail</em>: from a photo to the nearest one you have not seen.</li>
        </ul>
    </section>

    <section>
        <h2>Your libraries, read only</h2>
        <ul class="grid">
            <li><strong>Folders</strong> — one or several; RAW + JPEG pairs, edits and sidecars kept as one photo.</li>
            <li><strong>Apple Photos</strong> — its database and files, and what is only in iCloud, on a Mac.</li>
            <li><strong>Immich</strong> — its timeline through its API; its key never reaches the browser.</li>
        </ul>
        <p>
            Perceptrail never writes to a library: what it makes — the database, previews, videos the
            browser plays — lives in its own data directory.
        </p>
    </section>

    <section>
        <h2>Made for big libraries</h2>
        <ul class="grid">
            <li>A justified grid that keeps its place on a resize, over tens of thousands of photos.</li>
            <li>The list cached in the browser and synced by its changes; several tabs at once.</li>
            <li>Previews in WebP, videos as H.264 any browser plays, HDR tone mapped — made in the background.</li>
            <li>Photos, videos and Live Photos in one sheet; the info panel says what each perceptor knows.</li>
        </ul>
    </section>

    <section id="install">
        <h2>Install</h2>
        <div class="channels" role="tablist" aria-label="Channel">
            {#each Object.values(installs) as install (install.channel)}
                <button type="button" role="tab" aria-selected={shown === install.channel}
                        onclick={() => (shown = install.channel)}
                        data-cta="channel-{install.channel}" data-cta-placement="install">
                    {install.channel === 'stable' ? 'Stable' : 'Develop'}
                </button>
            {/each}
        </div>
        <p class="muted">
            {#if shown === 'stable'}
                The latest release (images tagged <code>latest</code>).
            {:else}
                Every merge, what is being tested (images tagged <code>develop</code>).
            {/if}
            In a directory of its own, Docker with compose:
        </p>
        <div class="code">
            <pre><code>{lines.join('\n')}</code></pre>
            <button type="button" onclick={copy} data-cta="copy-install" data-cta-placement={shown}>{copied ? 'Copied' : 'Copy'}</button>
        </div>
        <p class="muted">
            Then add your photos to <code>config.yml</code> and the mounts —
            <a href="{repo}/blob/{installs[shown].branch}/docs/install.md">the install guide</a>;
            a Mac with Apple Photos runs the native server.
            {#if shown !== channel}
                <a href={installs[shown].path} data-cta="channel-page" data-cta-placement="install">This channel's page</a>.
            {/if}
        </p>
    </section>

    <section>
        <h2>Status</h2>
        <p>
            Before its first release (0.2.0), running every day on its author's library. Open source
            (MPL 2.0) — the code, the roadmap and the design notes are
            <a href={repo} data-cta="github" data-cta-placement="status">on GitHub</a>.
        </p>
    </section>
</main>

<style>
    main {
        max-width: 64rem;
        margin: 0 auto;
        padding: 0 1.25rem;
    }

    .hero {
        padding: 5rem 0 3rem;
    }

    .kicker {
        margin: 0;
        color: var(--accent);
        font-size: 0.875rem;
        letter-spacing: 0.04em;
        text-transform: uppercase;
    }

    h1 {
        margin: 0.25rem 0 1rem;
        font-size: clamp(2.5rem, 7vw, 4.5rem);
        line-height: 1.05;
    }

    .lead {
        max-width: 40rem;
        margin: 0;
        color: var(--muted);
        font-size: 1.25rem;
    }

    .actions {
        display: flex;
        flex-wrap: wrap;
        gap: 0.75rem;
        margin-top: 2rem;
    }

    .button {
        display: inline-block;
        padding: 0.6rem 1.25rem;
        border: 1px solid var(--line);
        border-radius: 0.5rem;
        color: var(--text);
        text-decoration: none;
    }

    .button.primary {
        border-color: var(--accent);
        background: var(--accent);
        color: #1b1f3b;
        font-weight: 600;
    }

    section {
        padding: 2.5rem 0;
        border-top: 1px solid var(--line);
    }

    h2 {
        margin: 0 0 1rem;
        font-size: 1.5rem;
    }

    .grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
        gap: 1rem;
        margin: 1rem 0;
        padding: 0;
        list-style: none;
    }

    .grid li {
        padding: 1rem;
        border: 1px solid var(--line);
        border-radius: 0.5rem;
        background: var(--surface);
        color: var(--muted);
    }

    .grid strong {
        color: var(--text);
    }

    .muted {
        color: var(--muted);
    }

    .channels {
        display: inline-flex;
        border: 1px solid var(--line);
        border-radius: 0.5rem;
        overflow: hidden;
    }

    .channels button {
        padding: 0.4rem 1rem;
        border: 0;
        background: transparent;
        color: var(--muted);
        font: inherit;
        cursor: pointer;
    }

    .channels button[aria-selected='true'] {
        background: var(--surface);
        color: var(--accent);
    }

    .code {
        position: relative;
    }

    pre {
        margin: 0;
        padding: 1rem 5rem 1rem 1rem; /* the copy button stays off the lines */
        overflow-x: auto;
        border: 1px solid var(--line);
        border-radius: 0.5rem;
        background: #0c0e1f;
    }

    .code button {
        position: absolute;
        top: 0.5rem;
        right: 0.5rem;
        padding: 0.25rem 0.75rem;
        border: 1px solid var(--line);
        border-radius: 0.375rem;
        background: var(--surface);
        color: var(--text);
        font: inherit;
        font-size: 0.8rem;
        cursor: pointer;
    }
</style>
