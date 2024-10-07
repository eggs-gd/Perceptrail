import {Photo} from "react-photo-album";
import {Callback} from "yet-another-react-lightbox";

export interface CursorProps {
    photos: Photo[],
    index: number,
    setIndex: Callback<number>
}
