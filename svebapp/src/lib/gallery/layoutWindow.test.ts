import {describe, expect, it} from 'vitest';
import type {LayoutItem} from '$lib/stores';
import {findAnchor} from './layoutWindow';

const item = (guid: string, x: number, y: number, w: number, h: number) =>
    ({guid, x, y, w, h, bottom: y + h}) as LayoutItem;

// Two rows of three, 100 high, a gap of 8
const items = [
    item('a', 0, 0, 100, 100), item('b', 108, 0, 100, 100), item('c', 216, 0, 100, 100),
    item('d', 0, 108, 100, 100), item('e', 108, 108, 100, 100), item('f', 216, 108, 100, 100),
];

describe('findAnchor', () => {
    it('an edge pins the edge, no photo', () => {
        expect(findAnchor(items, {top: 0, height: 200, width: 316}, 'top')).toEqual({mode: 'top'});
    });

    it('the photo under the middle of the screen, and where in it', () => {
        // the middle: y 154 (row 2, 46 into it), x 158 (inside e)
        expect(findAnchor(items, {top: 54, height: 200, width: 316}, null)).toEqual({mode: 'center', guid: 'e', ratio: 0.46});
    });

    it('the middle in a gap between photos: the nearest one', () => {
        // x 105: 5 px right of d, 3 px left of e
        expect(findAnchor(items, {top: 54, height: 200, width: 210}, null)?.guid).toBe('e');
        // x 103: 3 px right of d, 5 px left of e
        expect(findAnchor(items, {top: 54, height: 200, width: 206}, null)?.guid).toBe('d');
    });

    it('below every photo: none', () => {
        expect(findAnchor(items, {top: 500, height: 200, width: 316}, null)).toBeUndefined();
    });
});
