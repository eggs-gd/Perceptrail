// The perceptors — each gives the sheet its order (a view of the library) — and the
// gallery's choices about them. The view itself lives in the URL (/v/<slug>?at=<guid>);
// only the pinned side panel is kept in the browser.
import {PUBLIC_API_PATH} from '$env/static/public';
import {getLogger} from '$lib/logger';

const logger = getLogger();
const KEY = 'perceptrail.gallery';

/** GET /perceptors */
export interface PerceptorView {
    /** The view's name in URLs */
    slug: string;
    title: string;
    /** SVG markup: shown as a mask (no scripts run), coloured by the button */
    icon: string;
    help: string;
    relative: boolean;
}

/** A switch the gallery carries out: the order around a photo */
export interface SwitchRequest {
    view: string;
    seq: number;
}

function loadPinned(): boolean {
    try {
        return JSON.parse(localStorage.getItem(KEY) ?? '{}').pinned ?? false;
    } catch {
        return false;
    }
}

export const perceptors = $state({
    list: [] as PerceptorView[],
    /** The view the sheet is laid out in (its order applied); '' until the first */
    active: '',
    /** The side panel is always shown and takes its width from the photos */
    pinned: loadPinned(),
    request: undefined as SwitchRequest | undefined,
});

/** The URL of a view, around a photo */
export function viewHref(slug: string, at?: string): string {
    return `/v/${encodeURIComponent(slug)}` + (at ? `?at=${encodeURIComponent(at)}` : '');
}

/** The URL of the viewer on a photo, in a view */
export function photoHref(slug: string, guid: string): string {
    return `/v/${encodeURIComponent(slug)}/${encodeURIComponent(guid)}`;
}

/** Loads the perceptors (the views); the URL says which one the sheet is in */
export async function loadPerceptors() {
    try {
        const response = await fetch(`${PUBLIC_API_PATH}/perceptors`);
        perceptors.list = await response.json();
    } catch (error) {
        logger.error('perceptors', error);
    }
}

/** A new switch request: only the latest one counts */
export function nextRequest(view: string): SwitchRequest {
    perceptors.request = {view, seq: (perceptors.request?.seq ?? 0) + 1};
    return perceptors.request;
}

/**
 * The gallery applied (or failed) a switch: only the latest request counts. Returns
 * whether the view is now active; a failed one leaves the previous view active — the
 * sheet is still in its order.
 */
export function orderApplied(request: SwitchRequest, ok: boolean): boolean {
    if (request.seq !== perceptors.request?.seq) return false;
    if (!ok) {
        logger.error('perceptor order not applied', request.view);
        return false;
    }
    perceptors.active = request.view;
    return true;
}

export function togglePinned() {
    perceptors.pinned = !perceptors.pinned;
    try {
        localStorage.setItem(KEY, JSON.stringify({pinned: perceptors.pinned}));
    } catch {
        // not kept: still switched for this visit
    }
}

/** An SVG icon as a CSS mask: the button's colour paints it, no script can run */
export function iconMask(svg: string): string {
    return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
}
