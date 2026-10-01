// The perceptors — each gives the sheet its order (a view of the library) — and the
// gallery's choices about them: the active one and the pinned side panel, kept per browser.
import {PUBLIC_API_PATH} from '$env/static/public';
import {getLogger} from '$lib/logger';

const logger = getLogger();
const KEY = 'perceptrail.gallery';

/** GET /perceptors */
export interface PerceptorView {
    name: string;
    title: string;
    /** SVG markup: shown as a mask (no scripts run), coloured by the button */
    icon: string;
    help: string;
    relative: boolean;
}

/** A switch the gallery carries out: the order around a photo */
export interface SwitchRequest {
    perceptor: string;
    /** Keep this photo in view; none: the photo in the middle of the screen */
    anchor?: string;
    seq: number;
}

interface Saved {
    active?: string;
    pinned?: boolean;
}

function load(): Saved {
    try {
        return JSON.parse(localStorage.getItem(KEY) ?? '{}');
    } catch {
        return {};
    }
}

const saved = load();

export const perceptors = $state({
    list: [] as PerceptorView[],
    /** The perceptor the sheet is in; '' until the list is loaded */
    active: '',
    /** The side panel is always shown and takes its width from the photos */
    pinned: saved.pinned ?? false,
    request: undefined as SwitchRequest | undefined,
});

function save() {
    try {
        localStorage.setItem(KEY, JSON.stringify({active: perceptors.active, pinned: perceptors.pinned}));
    } catch {
        // not kept: still switched for this visit
    }
}

/** Loads the perceptors and puts the sheet into the saved one (else the first: date) */
export async function loadPerceptors() {
    try {
        const response = await fetch(`${PUBLIC_API_PATH}/perceptors`);
        perceptors.list = await response.json();
    } catch (error) {
        logger.error('perceptors', error);
        return;
    }
    const first = perceptors.list.find((p) => p.name === saved.active) ?? perceptors.list[0];
    if (first) switchPerceptor(first.name);
}

/**
 * The sheet in another (or the same) perceptor's order, around a photo. The button
 * lights up (and is kept) once the order is applied — see orderApplied
 */
export function switchPerceptor(name: string, anchor?: string) {
    perceptors.request = {perceptor: name, anchor, seq: (perceptors.request?.seq ?? 0) + 1};
}

/**
 * The gallery applied (or failed) a switch: only the latest request counts; a failed
 * one leaves the previous perceptor active — the sheet is still in its order
 */
export function orderApplied(request: SwitchRequest, ok: boolean) {
    if (request.seq !== perceptors.request?.seq) return;
    if (!ok) {
        logger.error('perceptor order not applied', request.perceptor);
        return;
    }
    perceptors.active = request.perceptor;
    save();
}

export function togglePinned() {
    perceptors.pinned = !perceptors.pinned;
    save();
}

/** An SVG icon as a CSS mask: the button's colour paints it, no script can run */
export function iconMask(svg: string): string {
    return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
}
