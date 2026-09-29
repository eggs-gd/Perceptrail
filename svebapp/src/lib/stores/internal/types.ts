export interface Item {
    id: number;
    guid: string;
    mimeType: string;
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
}

/** Single record next to the layout: what the page needs besides the visible items */
export interface LayoutMeta {
    key: 'layout';
    /** Increases with every full relayout (resize) */
    rev: number;
    width: number;
    /** Total gallery height, px */
    height: number;
    count: number;
    /** Item to keep in view across the relayout of this rev: its new position */
    anchor?: {guid: string, y: number, h: number};
}
