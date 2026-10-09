import {PUBLIC_API_PATH} from '$env/static/public';
import {SvelteMap} from 'svelte/reactivity';
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
    return r.codec ? `${mime}; codecs="${fullCodec(r.codec)}"` : mime;
}

// The viewer picks from what is here first. Our comfortable size is the preview's
// (a long side of ~2048 px, Photos' medium rendition): the smallest local image that
// covers it — or the full size, if that is smaller — is shown as it is, no 360 px
// first. As big as the full size: that is the Original (its switch is lit); bigger
// is asked for with the switch.
const COMFORT_PX = 2048;
// Sizes from different sources round differently (an edit's render, the DB's size)
const FULL_SLACK = 0.98;

const longSide = (r: {w?: number, h?: number}) => Math.max(r.w ?? 0, r.h ?? 0);

/** The images the viewer may show here: the edit if there is one, else the stills
 * and an original every browser shows — smallest first, sized only */
function localImages(asset: Asset): Rendition[] {
    const images = asset.edit.length ? [...asset.edit] : [...asset.stills];
    if (!asset.edit.length && asset.original && VIEWABLE_IMAGES.has(asset.original.mime)) images.push(asset.original);
    return images.filter((r) => VIEWABLE_IMAGES.has(r.mime) && r.w && r.h)
        .sort((a, b) => longSide(a) - longSide(b));
}

/** A video's original, here and playable: shown at once, as the Original */
export function localFullVideo(asset: Asset): Rendition | undefined {
    const o = asset.original;
    return asset.kind === 'video' && o?.mime.startsWith('video/') && canPlayVideo(o) ? o : undefined;
}

// The client's guess, the server's veto: the full resolution came from Photos for
// these assets (the viewer's Original); the server learns it on its next pass. Kept
// with the asset as the server sent it: a copy that differs (the server processed
// it) is the server's word, the guess no longer applies. Lost on reload.
const fetchedFull = new SvelteMap<string, string>();

/** The viewer got the asset's full resolution from Photos: its cloud goes now */
export function fullFetched(guid: string, asset: Asset) {
    fetchedFull.set(guid, JSON.stringify(asset));
}

/**
 * The full resolution is here (the tile's cloud says when not): a video's original;
 * for an image, any file of the asset as big as the full size — the original, the
 * edit's render, a full-size derivative; or it just came from Photos (fullFetched)
 */
export function fullHere(asset: Asset, guid?: string): boolean {
    const guess = guid ? fetchedFull.get(guid) : undefined;
    if (guess && guess === JSON.stringify(asset)) return true;
    if (asset.kind === 'video') return !!asset.original?.mime.startsWith('video/');
    if (!asset.full) return !!asset.original;
    const full = longSide(asset.full) * FULL_SLACK;
    return [...asset.edit, ...asset.stills, ...(asset.original ? [asset.original] : [])]
        .some((r) => longSide(r) >= full);
}

/**
 * What the viewer shows of an image when it opens: the smallest local image that
 * covers our comfortable size (or the full size, if smaller); full — it is as big as
 * the full size (the Original switch is lit); ask — nothing here is that big, the
 * medium rendition is asked for (Photos)
 */
export function viewerChoice(asset: Asset): {image?: Rendition, full: boolean, ask: boolean} {
    const images = localImages(asset);
    const fullLong = asset.full ? longSide(asset.full) : longSide(images.at(-1) ?? {});
    const want = Math.min(COMFORT_PX, fullLong || COMFORT_PX) * FULL_SLACK;
    const image = images.find((r) => longSide(r) >= want) ?? images.at(-1);
    const size = image ? longSide(image) : 0;
    return {image, full: !!image && fullLong > 0 && size >= fullLong * FULL_SLACK, ask: size < want};
}

// Apple Photos on demand (the server's rendition.go): asked for when needed — the
// medium when the viewer opens, the hover when a moving tile is pointed at; the
// file comes when Photos has downloaded it (~1 s), 404 if it cannot

/** The viewer's rendition: the image (~2048 px or the edit), a video's 720p — H.264
 * only (?hevc=0) for a browser that plays no HEVC */
export function mediumUrl(asset: Asset): string | undefined {
    if (!asset.onDemand) return undefined;
    const url = `${PUBLIC_API_PATH}${asset.onDemand.medium}`;
    return asset.kind === 'video' && !playsHevc() ? `${url}${url.includes('?') ? '&' : '?'}hevc=0` : url;
}

/** A video's 360p or a Live Photo's motion, for a tile's hover */
export function hoverUrl(asset: Asset): string | undefined {
    return asset.onDemand?.hover && `${PUBLIC_API_PATH}${asset.onDemand.hover}`;
}

/**
 * The viewer's Original from Photos: the biggest of what the user sees — a photo's
 * (a Live Photo's photo's) current version, the edit, at full resolution as JPEG
 * (any browser shows it); a video's original file
 */
export function originalOnDemand(asset: Asset): string | undefined {
    return asset.onDemand?.original && `${PUBLIC_API_PATH}${asset.onDemand.original}`;
}

let hevc: boolean | undefined;
function playsHevc(): boolean {
    if (typeof document === 'undefined') return false;
    hevc ??= document.createElement('video').canPlayType(`video/mp4; codecs="${fullCodec('hvc1')}"`) !== '';
    return hevc;
}

/**
 * A codec as the browser wants to be asked: Chrome answers "" for a bare "hvc1" and
 * "probably" for "hvc1.1.6.L93.B0" (and plays it). The server knows only the FourCC
 * (exiftool's CompressorID): HEVC is asked as Main profile, level 3.1.
 */
function fullCodec(codec: string): string {
    return codec === 'hvc1' || codec === 'hev1' ? `${codec}.1.6.L93.B0` : codec;
}

/** Can this browser play the video (none on the server) */
export function canPlayVideo(r: Rendition): boolean {
    return typeof document !== 'undefined' && document.createElement('video').canPlayType(videoType(r)) !== '';
}

/**
 * The image the viewer's Original switch shows here (not from Photos): the biggest
 * of what the user sees — the biggest edit if there is one, else the original if it
 * is an image; a Live Photo whose original is its video: its biggest photo. Edits
 * and their history belong to the library, not to us: no unedited original over an
 * edit.
 */
export function originalImage(asset: Asset): Rendition | undefined {
    const bySize = (rs: Rendition[]) => [...rs].sort((a, b) => (b.w ?? 0) - (a.w ?? 0))[0];
    if (asset.edit.length) return bySize(asset.edit);
    if (asset.original?.mime.startsWith('image/')) return asset.original;
    if (asset.kind === 'live') return bySize(asset.stills);
    return undefined;
}

// Formats a browser may or may not show (HEIC: Safari yes, Chrome no): tested once
// per type by decoding a real file; anything else not viewable (RAW) is never shown
const MAYBE_VIEWABLE = new Set(['image/heic', 'image/heif', 'image/jxl', 'image/tiff', 'image/bmp']);
const shownByType = new Map<string, Promise<boolean>>();

/** Does this browser show the image: known formats at once, the rest tested once per type */
export function canShowImage(r: Rendition): Promise<boolean> {
    if (VIEWABLE_IMAGES.has(r.mime)) return Promise.resolve(true);
    if (!MAYBE_VIEWABLE.has(r.mime) || typeof Image === 'undefined') return Promise.resolve(false);
    let shown = shownByType.get(r.mime);
    if (!shown) {
        const img = new Image();
        img.src = assetUrl(r);
        shown = img.decode().then(() => true, () => false);
        shownByType.set(r.mime, shown);
    }
    return shown;
}
