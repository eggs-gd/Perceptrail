<script lang="ts">
    import ConsentBanner from '#lib/analytics/ConsentBanner.svelte';
    import {reopenConsent} from '#lib/analytics/consent.js';
    import '../app.css';

    let {children} = $props();
</script>

{@render children()}

<footer>
    <span>© {new Date().getFullYear()} earwiG Games</span>
    <a href="https://eggs.gd">eggs.gd</a>
    <a href="https://eggs.gd/privacy">Privacy</a>
    <button type="button" onclick={reopenConsent}>Cookie settings</button>
</footer>

<ConsentBanner privacyHref="https://eggs.gd/privacy"/>

<style>
    footer {
        display: flex;
        flex-wrap: wrap;
        gap: 0.5rem 1.5rem;
        align-items: center;
        max-width: 64rem;
        margin: 0 auto;
        padding: 2rem 1.25rem 3rem;
        border-top: 1px solid var(--line);
        color: var(--muted);
        font-size: 0.875rem;
    }

    footer a {
        color: var(--muted);
    }

    button {
        padding: 0;
        border: 0;
        background: none;
        color: var(--muted);
        font: inherit;
        text-decoration: underline;
        cursor: pointer;
    }
</style>
