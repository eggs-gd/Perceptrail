/** One file of an asset as the server offers it (see gontroller routes/asset.go) */
export interface Rendition {
    /** Relative to the API: /assets/<guid>/<id> */
    url: string;
    mime: string;
    w?: number;
    h?: number;
    /** A video's codec: avc1, hvc1, … */
    codec?: string;
}

/** Every file of an asset, by role: the client decides what to show when */
export type AssetKind = 'photo' | 'live' | 'video';

export interface Asset {
    /** What the asset is: the tile marks the moving ones */
    kind: AssetKind;
    /** A video's length, seconds */
    duration?: number;
    /** The source; may not be viewable (HEIC, RAW, HEVC). null: not local (iCloud) */
    original: Rendition | null;
    /** The user's edit, smallest first */
    edit: Rendition[];
    /** Viewable images, smallest first */
    stills: Rendition[];
    /** Videos of the asset (a Live Photo's video) */
    motion: Rendition[];
    /** A flip-book, in order (Apple's video frames) */
    frames: Rendition[];
}

export interface Item {
    id: number;
    guid: string;
    mimeType: string;
    /** What /assets/:guid serves (an image, or a playable video): picks <img> or <video> */
    previewMime: string;
    /** Every file of the asset; absent from an older server */
    asset?: Asset;
    width: number;
    height: number;
    date: Date;
}

export interface LayoutItem extends Item {
    order: number;
    scale: number;
    row: number;
    /** Position and size in the gallery, px */
    x: number;
    y: number;
    w: number;
    h: number;
    /** y + h: the bottom of the item's row (indexed — finds the first row in the window) */
    bottom: number;
}

/**
 * Written only by a full relayout. The visible-window query reads this record, so it
 * must not change per streamed photo (that would re-run the query for every photo).
 */
export interface LayoutMeta {
    key: 'layout';
    /** Increases with every full relayout (resize) */
    rev: number;
    width: number;
    /** Total gallery height right after this relayout, px */
    height: number;
    /** Item to keep in view across the relayout of this rev: its new position */
    anchor?: {guid: string, y: number, h: number};
}

/** Gallery size, updated with every batch of streamed photos; separate subscription */
export interface LayoutSize {
    key: 'size';
    /** The relayout this size belongs to */
    rev: number;
    height: number;
    count: number;
}
