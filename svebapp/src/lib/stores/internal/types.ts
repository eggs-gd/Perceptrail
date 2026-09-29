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
