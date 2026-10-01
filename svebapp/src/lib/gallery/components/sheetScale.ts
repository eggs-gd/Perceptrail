import type {SectionMark} from '$lib/stores';

/**
 * The side panel's scale: where on the track a place of the sheet is, and back.
 *
 * Not linear by the sheet's height: at every level the sections share their
 * parent's part of the track by the square root of their photo count (and at least
 * room for a label when that fits) — years, then the months inside a year; regions,
 * then the cities inside a region. Big sections shrink, small ones grow, the order
 * stays: two thirds of the library without a place no longer push the real places
 * into a corner, and one big city no longer hides the others of its region. No
 * section is special: "no value" is just a big one. Linear within the finest
 * section, so scrubbing is smooth.
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

/** A part of the sheet: from y0 to y1, photos [o0, o1) */
interface Span {
    y0: number;
    y1: number;
    o0: number;
    o1: number;
}

/**
 * marks: the sections (their first photo's y and order, by level); height and count:
 * the whole sheet; track: the panel's height; minShare: px a section gets at least
 */
export function sheetScale(marks: SectionMark[], height: number, count: number, track: number, minShare: number): SheetScale {
    if (!height || !track) {
        return {toTrack: () => 0, toSheet: () => 0};
    }
    const byLevel = new Map<number, SectionMark[]>();
    for (const m of marks) (byLevel.get(m.level) ?? byLevel.set(m.level, []).get(m.level)!).push(m);
    for (const list of byLevel.values()) list.sort((a, b) => a.y - b.y);
    const deepest = Math.max(-1, ...byLevel.keys());

    const segments: Segment[] = [];
    split({y0: 0, y1: height, o0: 0, o1: count}, 0, track, 0);
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

    /** Shares span's part of the track [t0, t1) among its sections of this level */
    function split(span: Span, t0: number, t1: number, level: number) {
        if (level > deepest) {
            segments.push({y0: span.y0, y1: span.y1, t0, t1});
            return;
        }
        const starts = (byLevel.get(level) ?? []).filter((m) => m.y >= span.y0 && m.y < span.y1);
        if (starts.length === 0) {
            split(span, t0, t1, level + 1);
            return;
        }
        // From one section's first photo to the next one's; a head before the first
        // mark (the rest of a parent section) is a part of its own
        const parts: Span[] = [];
        if (starts[0].y > span.y0) parts.push({y0: span.y0, y1: starts[0].y, o0: span.o0, o1: starts[0].order});
        starts.forEach((m, i) => {
            const next = starts[i + 1];
            parts.push({y0: m.y, y1: next ? next.y : span.y1, o0: m.order, o1: next ? next.order : span.o1});
        });

        const room = t1 - t0;
        const weights = parts.map((p) => Math.sqrt(Math.max(1, p.o1 - p.o0)));
        const total = weights.reduce((a, b) => a + b, 0);
        const floor = parts.length * minShare <= room / 2 ? minShare : 0;
        const free = room - floor * parts.length;
        let t = t0;
        parts.forEach((p, i) => {
            const share = floor + (free * weights[i]) / total;
            split(p, t, t + share, level + 1);
            t += share;
        });
    }
}

function within(v: number, a0: number, a1: number, b0: number, b1: number): number {
    return a1 > a0 ? b0 + ((v - a0) / (a1 - a0)) * (b1 - b0) : b0;
}
