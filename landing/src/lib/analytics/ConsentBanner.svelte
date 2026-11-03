<script lang="ts">
	import { onMount } from 'svelte';
	import { choose, consent, initConsent } from './consent';

	/** Link to the privacy policy; omitted when empty. */
	let { privacyHref = '' }: { privacyHref?: string } = $props();

	let ready = $state(false);

	onMount(() => {
		initConsent();
		ready = true;
	});
</script>

{#if ready && $consent === 'unset'}
	<aside class="consent" aria-label="Analytics cookies">
		<p>
			This site uses Google Analytics to count visits. Cookies are set only if you accept; without
			them it sends only cookieless pings.
			{#if privacyHref}<a href={privacyHref}>Privacy policy</a>{/if}
		</p>
		<div class="consent-actions">
			<button type="button" onclick={() => choose('denied')}>Decline</button>
			<button type="button" onclick={() => choose('granted')}>Accept</button>
		</div>
	</aside>
{/if}

<style>
	/* Colours come from --consent-* variables, mapped to each site's tokens in its global CSS. */
	.consent {
		position: fixed;
		z-index: 60;
		right: 1rem;
		bottom: calc(1rem + env(safe-area-inset-bottom, 0px));
		left: 1rem;
		max-width: 35rem;
		margin-left: auto;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1rem;
		padding: 0.85rem 1rem;
		border: 1px solid var(--consent-border, rgba(255, 255, 255, 0.2));
		border-radius: 0.5rem;
		background: var(--consent-bg, #1a1a1a);
		color: var(--consent-text, #eeeeee);
		font-size: 0.875rem;
		line-height: 1.45;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
	}

	p {
		flex: 1 1 15rem;
		margin: 0;
		color: var(--consent-muted, #aaaaaa);
	}

	p a {
		color: inherit;
		text-decoration: underline;
	}

	.consent-actions {
		display: flex;
		gap: 0.5rem;
	}

	button {
		min-height: 2.25rem;
		padding: 0 1rem;
		border: 1px solid var(--consent-border, rgba(255, 255, 255, 0.2));
		border-radius: 0.375rem;
		background: transparent;
		color: var(--consent-text, #eeeeee);
		font: inherit;
		cursor: pointer;
	}

	button:hover {
		border-color: var(--consent-accent, currentColor);
	}

	button:focus-visible {
		outline: 2px solid var(--consent-accent, currentColor);
		outline-offset: 2px;
	}
</style>
