import React from "react";
import PhotoAlbum from "react-photo-album";
import "react-photo-album/styles.css";
import {IGridProps} from "./IViewProps";

export default function GridView(props: Readonly<IGridProps>) {
    return (
        <PhotoAlbum
            layout="rows"
            photos={props.photos}
            padding={0}
            spacing={10}
            targetRowHeight={850}
            onClick={({index}) => {
                props.setIndex(index);
                props.openSingle(true)
            }}
            sizes={{
                size: "1168px",
                sizes: [
                    {
                        viewport: "(max-width: 1200px)",
                        size: "calc(100vw - 32px)",
                    },
                ],
            }}
        />
    );
}
