// The viewer's switches the user sets once: kept across items and visits (this
// browser only)

const KEY = 'perceptrail.viewer';

interface ViewerPrefs {
    /** Play a Live Photo's motion when it opens */
    autoplayLive: boolean;
    /** Start a video when it opens */
    autoplayVideo: boolean;
    /** Stretch an image smaller than the screen to fit it (off: its own size at most) */
    stretchSmall: boolean;
}

function load(): Partial<ViewerPrefs> {
    try {
        return JSON.parse(localStorage.getItem(KEY) ?? '{}');
    } catch {
        return {}; // no storage (server, private mode): the defaults
    }
}

const saved = load();

export const viewerPrefs: ViewerPrefs = $state({
    autoplayLive: saved.autoplayLive ?? true,
    autoplayVideo: saved.autoplayVideo ?? true,
    stretchSmall: saved.stretchSmall ?? false,
});

export function toggleViewerPref(key: keyof ViewerPrefs) {
    viewerPrefs[key] = !viewerPrefs[key];
    try {
        localStorage.setItem(KEY, JSON.stringify(viewerPrefs));
    } catch {
        // not kept: still switched for this visit
    }
}
