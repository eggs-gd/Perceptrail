import React from "react";

import Lightbox from "yet-another-react-lightbox";
import "yet-another-react-lightbox/styles.css";

import Video from "yet-another-react-lightbox/plugins/video";
import "yet-another-react-lightbox/plugins/thumbnails.css";
import {CursorProps} from "./CursorProps";

export default function Focused(props: Readonly<CursorProps>) {
    return (
        <Lightbox
            slides={props.photos?.map(p => {
                p.width *= 100;
                p.height *= 100;
                return p;
            })}
            open={props.index >= 0}
            index={props.index}
            close={() => {}}
            on = {{view: ({index:i}) => props.setIndex(i)}}
            // enable optional lightbox plugins
            plugins={[
                Video, // Fullscreen, Slideshow, Thumbnails, Zoom
            ]}
        />
    );
}
