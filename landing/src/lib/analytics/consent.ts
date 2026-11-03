import { writable } from 'svelte/store';

/**
 * Consent state for the Google tag. The tag itself lives in app.html, where it starts with
 * analytics storage denied (Consent Mode) and applies a stored 'granted' before the first page
 * view. app.html also defines window.setAnalyticsConsent, which stores the choice and tells the tag.
 * This module only drives the banner.
 */
const STORAGE_KEY = 'analyticsConsent';

export type Consent = 'granted' | 'denied' | 'unset';

declare global {
	interface Window {
		setAnalyticsConsent?: (choice: 'granted' | 'denied') => void;
	}
}

const read = (): Consent => {
	try {
		const value = localStorage.getItem(STORAGE_KEY);
		return value === 'granted' || value === 'denied' ? value : 'unset';
	} catch {
		return 'unset';
	}
};

/** What the visitor chose; `unset` means the banner is showing. */
export const consent = writable<Consent>('unset');

/** Shows the banner on a first visit; a stored choice was already applied by app.html. */
export const initConsent = () => consent.set(read());

export function choose(choice: 'granted' | 'denied') {
	window.setAnalyticsConsent?.(choice);
	consent.set(choice);
}

/** Shows the banner again so the visitor can change the choice. */
export const reopenConsent = () => consent.set('unset');
