import {describe, expect, it} from 'vitest';
import type {Asset, Rendition} from '#lib/stores/index.js';
import {fallbackImage, imageOrientation, mediumUrl, pictureSources} from './asset';

const image = (url: string, mime: string, w?: number): Rendition => ({url, mime, w, h: w});
const asset = (over: Partial<Asset>): Asset =>
    ({kind: 'photo', original: null, edit: [], stills: [], motion: [], frames: [], ...over}) as Asset;

describe('pictureSources', () => {
    it('the stills, a source per type, best format first, each sized', () => {
        const stills = [image('/s400.jpg', 'image/jpeg', 400), image('/s400.webp', 'image/webp', 400), image('/s1600.webp', 'image/webp', 1600)];
        expect(pictureSources(asset({stills}))).toEqual([
            {type: 'image/webp', srcset: '/api/s400.webp 400w, /api/s1600.webp 1600w'},
            {type: 'image/jpeg', srcset: '/api/s400.jpg 400w'},
        ]);
    });

    it('with an original every browser shows: it is offered too, unsized as it is', () => {
        const a = asset({stills: [image('/s400.jpg', 'image/jpeg', 400)], original: image('/orig.jpg', 'image/jpeg')});
        expect(pictureSources(a)).toEqual([{type: 'image/jpeg', srcset: '/api/s400.jpg 400w, /api/orig.jpg'}]);
        expect(a.stills).toHaveLength(1);
    });

    it('an original a browser may not show (HEIC, RAW) is left to the stills', () => {
        const a = asset({stills: [image('/s400.webp', 'image/webp', 400)], original: image('/orig.heic', 'image/heic', 4000)});
        expect(pictureSources(a)).toEqual([{type: 'image/webp', srcset: '/api/s400.webp 400w'}]);
    });

    it('the edit is what the user sees: no stills, no original', () => {
        const a = asset({
            edit: [image('/e800.webp', 'image/webp', 800)],
            stills: [image('/s400.webp', 'image/webp', 400)],
            original: image('/orig.jpg', 'image/jpeg', 3000),
        });
        expect(pictureSources(a)).toEqual([{type: 'image/webp', srcset: '/api/e800.webp 800w'}]);
    });

    it('a type outside the preference goes last; what is not an image is no source', () => {
        const stills = [image('/s.heic', 'image/heic', 400), image('/s.png', 'image/png', 400), image('/v.mp4', 'video/mp4', 400)];
        expect(pictureSources(asset({stills})).map((s) => s.type)).toEqual(['image/png', 'image/heic']);
        expect(pictureSources(asset({}))).toEqual([]);
    });
});

describe('fallbackImage', () => {
    it('the biggest image every browser shows, the asset left as it was', () => {
        const stills = [image('s400', 'image/webp', 400), image('heic', 'image/heic', 4000), image('s1600', 'image/webp', 1600)];
        const a = asset({stills, original: image('jpg', 'image/jpeg', 3000)});
        expect(fallbackImage(a)?.url).toBe('jpg');
        expect(a.stills.map((r) => r.url)).toEqual(['s400', 'heic', 's1600']);
    });

    it('none viewable: none', () => {
        expect(fallbackImage(asset({original: image('raw', 'image/x-nikon-nef', 6000)}))).toBeUndefined();
    });
});

describe('imageOrientation', () => {
    it('"none" when the server says a file\'s Orientation tag is stale; else as the files say', () => {
        const stale = {...image('jpg', 'image/jpeg', 3000), orientation: 'none' as const};
        expect(imageOrientation(asset({stills: [image('s400', 'image/webp', 400)], original: stale}))).toBe('none');
        expect(imageOrientation(asset({original: image('jpg', 'image/jpeg', 3000)}))).toBeUndefined();
    });
});

describe('mediumUrl', () => {
    it('a library asset: its medium from the API; none without', () => {
        expect(mediumUrl(asset({onDemand: {medium: '/items/g/rendition/medium?v=7'}} as Partial<Asset>))).toBe('/api/items/g/rendition/medium?v=7');
        expect(mediumUrl(asset({}))).toBeUndefined();
    });

    it('a video where HEVC does not play (no browser here): asked without it', () => {
        expect(mediumUrl(asset({kind: 'video', onDemand: {medium: '/items/g/rendition/medium?v=7'}} as Partial<Asset>)))
            .toBe('/api/items/g/rendition/medium?v=7&hevc=0');
        expect(mediumUrl(asset({kind: 'video', onDemand: {medium: '/items/g/rendition/medium'}} as Partial<Asset>)))
            .toBe('/api/items/g/rendition/medium?hevc=0');
    });
});
