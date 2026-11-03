import {beforeEach, describe, expect, it, vi} from 'vitest';
import {get} from 'svelte/store';
import {choose, consent, initConsent, reopenConsent} from './consent';

// The browser's storage and the tag's hook (app.html), as a page has them
const stored = new Map<string, string>();
beforeEach(() => {
    stored.clear();
    vi.stubGlobal('localStorage', {
        getItem: (k: string) => stored.get(k) ?? null,
        setItem: (k: string, v: string) => void stored.set(k, v),
    });
    vi.stubGlobal('setAnalyticsConsent', vi.fn((c: string) => stored.set('analyticsConsent', c)));
});

describe('consent', () => {
    it('a first visit shows the banner; a stored choice does not', () => {
        initConsent();
        expect(get(consent)).toBe('unset');
        stored.set('analyticsConsent', 'denied');
        initConsent();
        expect(get(consent)).toBe('denied');
    });

    it('a choice goes to the tag and hides the banner; settings show it again', () => {
        choose('granted');
        expect(globalThis.setAnalyticsConsent).toHaveBeenCalledWith('granted');
        expect(get(consent)).toBe('granted');
        reopenConsent();
        expect(get(consent)).toBe('unset');
    });

    it('no storage (a private window): the banner shows', () => {
        vi.stubGlobal('localStorage', {getItem: () => { throw new Error('denied'); }});
        initConsent();
        expect(get(consent)).toBe('unset');
    });
});
