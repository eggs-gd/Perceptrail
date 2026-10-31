import {describe, expect, it} from 'vitest';
import type {Asset, Rendition} from '#lib/stores/index.js';
import {fallbackImage, mediumUrl} from './asset';

const image = (url: string, mime: string, w?: number): Rendition => ({url, mime, w, h: w});
const asset = (over: Partial<Asset>): Asset =>
    ({kind: 'photo', original: null, edit: [], stills: [], motion: [], frames: [], ...over}) as Asset;

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
