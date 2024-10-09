export interface Item {
    src: string;

    width: number;
    height: number;
}

export interface ItemOrig extends Item {
    index: number;
    ratio: number;
}

export interface ItemScaled extends ItemOrig {
    scaledWidth: number;
    scaledHeight: number;

    scaledWidthPc: number;

    isLastInRow: boolean;
    isLastRow: boolean;
}

export interface LayoutParams {
    images: Item[];
    containerWidth: number;
    targetHeight: number;
    gutter: number;

    seekLimit?(containerWidth: number, targetRowHeight: number): number;

    byRow?: boolean;
}
