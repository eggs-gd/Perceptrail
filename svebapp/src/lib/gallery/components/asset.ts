import {PUBLIC_API_PATH} from '$env/static/public';
import type {Asset, Rendition} from '$lib/stores';

// What a browser surely shows; anything else only in the formats it claims
const IMAGE_PREFERENCE = ['image/avif', 'image/webp', 'image/jpeg', 'image/png', 'image/gif'];
const VIEWABLE_IMAGES = new Set(IMAGE_PREFERENCE);

export const assetUrl = (r: Rendition) => `${PUBLIC_API_PATH}${r.url}`;

export interface PictureSource {
    type: string;
    srcset: string;
}

/**
 * The images to offer a <picture>: the edit if there is one (what the user sees in
 * Photos), else the stills — plus the original when every browser shows it. Grouped
 * by type (a <source> each, best format first), sized (srcset "w"): the browser
 * picks the format and the size itself.
 */
export function pictureSources(asset: Asset): PictureSource[] {
    const images = [...(asset.edit.length ? asset.edit : asset.stills)];
    if (!asset.edit.length && asset.original && VIEWABLE_IMAGES.has(asset.original.mime)) {
        images.push(asset.original);
    }
    const byType = new Map<string, Rendition[]>();
    for (const r of images) {
        if (!r.mime.startsWith('image/')) continue;
        byType.set(r.mime, [...(byType.get(r.mime) ?? []), r]);
    }
    const rank = (t: string) => {
        const i = IMAGE_PREFERENCE.indexOf(t);
        return i < 0 ? IMAGE_PREFERENCE.length : i;
    };
    return [...byType.entries()]
        .sort(([a], [b]) => rank(a) - rank(b))
        .map(([type, rs]) => ({
            type,
            srcset: rs.map((r) => (r.w ? `${assetUrl(r)} ${r.w}w` : assetUrl(r))).join(', '),
        }));
}

/** Is there an image to show: the edit, a still, or an original every browser shows */
export function hasImage(asset: Asset): boolean {
    return asset.edit.length > 0 || asset.stills.length > 0
        || (!!asset.original && VIEWABLE_IMAGES.has(asset.original.mime));
}

/**
 * The biggest image the <picture> may show, with its size: the viewer shows no more
 * pixels than that (a small image is not stretched), fitted into the screen
 */
export function biggestImage(asset: Asset): Rendition | undefined {
    const images = [...(asset.edit.length ? asset.edit : asset.stills)];
    if (!asset.edit.length && asset.original && VIEWABLE_IMAGES.has(asset.original.mime)) {
        images.push(asset.original);
    }
    return images.filter((r) => r.w && r.h).sort((a, b) => b.w! - a.w!)[0];
}

/** The <img> fallback: the biggest image every browser shows */
export function fallbackImage(asset: Asset): Rendition | undefined {
    const images = [...asset.edit, ...asset.stills].filter((r) => VIEWABLE_IMAGES.has(r.mime));
    if (asset.original && VIEWABLE_IMAGES.has(asset.original.mime)) images.push(asset.original);
    return images.sort((a, b) => (b.w ?? 0) - (a.w ?? 0))[0];
}

export interface VideoSource {
    src: string;
    type: string;
}

/**
 * The videos of the asset (its motion, and the original if it is a video) as
 * <source>s with their codec: the browser skips what it cannot play (HEVC outside
 * Safari). QuickTime is offered as MP4 — the same container family, which Chrome
 * plays but does not claim as video/quicktime.
 */
export function videoSources(asset: Asset): VideoSource[] {
    const videos = [...asset.motion];
    if (asset.original?.mime.startsWith('video/')) videos.push(asset.original);
    return videos.map((r) => ({src: assetUrl(r), type: videoType(r)}));
}

/**
 * Is the asset's video here (its motion, or a video original) — not only in the
 * cloud (an iCloud library keeps just stills and a few frames of it)
 */
export function hasLocalVideo(asset: Asset): boolean {
    return asset.motion.length > 0 || !!asset.original?.mime.startsWith('video/');
}

/** 24 → 0:24, 83 → 1:23, 3723 → 1:02:03 */
export function formatDuration(seconds: number): string {
    const s = Math.round(seconds);
    const pad = (n: number) => String(n).padStart(2, '0');
    const h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60;
    return h ? `${h}:${pad(m)}:${pad(s % 60)}` : `${m}:${pad(s % 60)}`;
}

/** Only the videos this browser says it can play (none on the server) */
export function playableVideos(asset: Asset): VideoSource[] {
    if (typeof document === 'undefined') return [];
    const probe = document.createElement('video');
    return videoSources(asset).filter((v) => probe.canPlayType(v.type) !== '');
}

function videoType(r: Rendition): string {
    const mime = r.mime === 'video/quicktime' ? 'video/mp4' : r.mime;
    return r.codec ? `${mime}; codecs="${r.codec}"` : mime;
}
