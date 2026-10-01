import type {SectionMark} from '$lib/stores';

/**
 * The side panel's scale: where on the track a place of the sheet is, and back.
 *
 * Not linear by the sheet's height: every coarsest section gets a share of the track
 * by the square root of its photo count (and at least room for its label). Big
 * sections shrink, small ones grow, the order stays — two thirds of the library
 * without a place no longer push the real places into a corner, and a dense year no
 * longer squeezes the others. No section is special: "no value" is just a big one.
 * Within a section the scale is linear, so scrubbing is smooth.
 */

interface Segment {
    /** The sheet's span, px */
    y0: number;
    y1: number;
    /** The track's span, px */
    t0: number;
    t1: number;
}

export interface SheetScale {
    toTrack(y: number): number;
    toSheet(t: number): number;
}

/**
 * marks: the sections (their first photo's y and order); height and count: the
 * whole sheet; track: the panel's height; minShare: px a section gets at least
 */
export function sheetScale(marks: SectionMark[], height: number, count: number, track: number, minShare: number): SheetScale {
    const linear: SheetScale = {
        toTrack: (y) => (height ? (y / height) * track : 0),
        toSheet: (t) => (track ? (t / track) * height : 0),
    };
    const starts = marks.filter((m) => m.level === 0).sort((a, b) => a.y - b.y);
    if (!height || !track || starts.length === 0) return linear;

    // Sections from their first photo to the next one's; photos above the first mark
    // (none, usually) form a section of their own
    const spans: {y0: number, y1: number, n: number}[] = [];
    if (starts[0].y > 0) spans.push({y0: 0, y1: starts[0].y, n: starts[0].order});
    starts.forEach((m, i) => {
        const next = starts[i + 1];
        spans.push({y0: m.y, y1: next ? next.y : height, n: (next ? next.order : count) - m.order});
    });

    // Shares: room for a label each (when it fits), the rest by √count
    const weights = spans.map((s) => Math.sqrt(Math.max(1, s.n)));
    const total = weights.reduce((a, b) => a + b, 0);
    const floor = spans.length * minShare <= track / 2 ? minShare : 0;
    const free = track - floor * spans.length;
    const segments: Segment[] = [];
    let t = 0;
    spans.forEach((s, i) => {
        const share = floor + (free * weights[i]) / total;
        segments.push({y0: s.y0, y1: s.y1, t0: t, t1: t + share});
        t += share;
    });

    const within = (v: number, a0: number, a1: number, b0: number, b1: number) =>
        a1 > a0 ? b0 + ((v - a0) / (a1 - a0)) * (b1 - b0) : b0;
    return {
        toTrack(y) {
            const s = segments.find((g) => y < g.y1) ?? segments[segments.length - 1];
            return within(Math.min(Math.max(y, s.y0), s.y1), s.y0, s.y1, s.t0, s.t1);
        },
        toSheet(p) {
            const s = segments.find((g) => p < g.t1) ?? segments[segments.length - 1];
            return within(Math.min(Math.max(p, s.t0), s.t1), s.t0, s.t1, s.y0, s.y1);
        },
    };
}
