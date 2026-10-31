const MIN_ROW_HEIGHT = 118;
const MAX_ROW_HEIGHT = 260;

function clamp(min: number, value: number, max: number): number {
    return Math.max(min, Math.min(max, value));
}

/**
 * A target row height in CSS pixels. CSS px already accounts for most device
 * density, but high-DPR narrow screens still benefit from slightly smaller tiles.
 */
export function galleryRowHeight(width: number, pixelRatio: number): number {
    if (!width) return 180;
    const density = clamp(1, pixelRatio || 1, 3);
    const byWidth = 56 + Math.sqrt(width) * 5;
    const denseScreenBias = 1 + (density - 1) * 0.08;
    return Math.round(clamp(MIN_ROW_HEIGHT, byWidth / denseScreenBias, MAX_ROW_HEIGHT));
}
