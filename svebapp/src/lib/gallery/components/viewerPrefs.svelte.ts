// The viewer's switches the user sets once: kept across items and visits (this
// browser only)

const KEY = 'perceptrail.viewer';

/** A Live Photo's motion when it opens: not played, played once, played in a loop */
export type LiveMode = 'off' | 'once' | 'loop';
const LIVE_MODES: LiveMode[] = ['off', 'once', 'loop'];

interface ViewerPrefs {
    liveMode: LiveMode;
    /** Start a video when it opens */
    autoplayVideo: boolean;
    /** Stretch an image smaller than the screen to fit it (off: its own size at most) */
    stretchSmall: boolean;
    /** The info panel is open — kept photo to photo and between visits */
    infoOpen: boolean;
}

function load(): Partial<ViewerPrefs> & {autoplayLive?: boolean} {
    try {
        return JSON.parse(localStorage.getItem(KEY) ?? '{}');
    } catch {
        return {}; // no storage (server, private mode): the defaults
    }
}

const saved = load();

export const viewerPrefs: ViewerPrefs = $state({
    // Kept before as a switch: off stays off, on is once
    liveMode: saved.liveMode ?? (saved.autoplayLive === false ? 'off' : 'once'),
    autoplayVideo: saved.autoplayVideo ?? true,
    stretchSmall: saved.stretchSmall ?? false,
    infoOpen: saved.infoOpen ?? false,
});

type Switch = Exclude<keyof ViewerPrefs, 'liveMode'>;

export function toggleViewerPref(key: Switch) {
    viewerPrefs[key] = !viewerPrefs[key];
    save();
}

/** One button, three states: off → once → loop → off */
export function cycleLiveMode() {
    viewerPrefs.liveMode = LIVE_MODES[(LIVE_MODES.indexOf(viewerPrefs.liveMode) + 1) % LIVE_MODES.length];
    save();
}

function save() {
    try {
        localStorage.setItem(KEY, JSON.stringify(viewerPrefs));
    } catch {
        // not kept: still switched for this visit
    }
}
