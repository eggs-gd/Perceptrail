export interface Item {
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
}
