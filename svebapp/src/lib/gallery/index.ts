import {SvelteComponent} from "svelte";
import Gallery from './Gallery.svelte';

export type {Item, LayoutParams} from './types'

export interface IGalleryProps {
    images: Partial<HTMLImageElement>[];
    rowHeight?: number;
    gutter?: number;
    imageComponent?: typeof SvelteComponent;
}

export class TGallery extends SvelteComponent<
    IGalleryProps,
    {},
    {}
> {
}

export default Gallery;
