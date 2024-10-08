import {Photo} from "react-photo-album";
import {Callback} from "yet-another-react-lightbox";


export interface IViewProps {
    photos: Photo[],
    index: number,
}

export interface ICursorProps extends IViewProps {
    setIndex: Callback<number>,
}

export interface IGridProps extends ICursorProps {
    openSingle: Callback<boolean>,
}

export interface ISingleProps extends ICursorProps {
    isSingleOpened: boolean,
    openSingle: Callback<boolean>,
}
