import {describe, expect, it} from 'vitest';
import {sheetScale} from './sheetScale';

describe('sheetScale', () => {
    it('no sheet or no track: everything at 0', () => {
        const s = sheetScale([], 0, 0, 100, 10);
        expect([s.toTrack(50), s.toSheet(50)]).toEqual([0, 0]);
    });

    it('without sections: a straight scale, past the end clamped to it', () => {
        const s = sheetScale([], 1000, 10, 100, 10);
        expect([s.toTrack(0), s.toTrack(500), s.toTrack(1000), s.toTrack(5000)]).toEqual([0, 50, 100, 100]);
        expect([s.toSheet(0), s.toSheet(50), s.toSheet(100), s.toSheet(500)]).toEqual([0, 500, 1000, 1000]);
    });
});
