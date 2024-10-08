import React from "react";
import PhotoAlbum from "react-photo-album";
import "react-photo-album/styles.css";
import {CursorProps} from "./CursorProps";

export default function GridView(props: Readonly<CursorProps>) {
    return (
            <PhotoAlbum
                layout="rows"
                photos={props.photos}
                padding={0}
                spacing={10}
                targetRowHeight={850}
                onClick={({index}) => props.setIndex(index)}
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
