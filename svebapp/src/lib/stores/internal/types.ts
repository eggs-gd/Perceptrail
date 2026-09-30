export interface Item {
    id: number;
    guid: string;
    mimeType: string;
    /** What /assets/:guid serves (an image, or a playable video): picks <img> or <video> */
    previewMime: string;
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
