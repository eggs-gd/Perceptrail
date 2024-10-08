import React, {useState} from 'react';
import './App.css';
import {photos as photoSet} from './data/photos';

import GridView from "./gallery/GridView";
import SingleView from "./gallery/SingleView";
import TopBar from "./gallery/TopBar";
//import "@perseptrail/perceptors";
import {DatePerceptor, IPerceptor} from "@perseptrail/perceptors";


const enabledPerceptors: IPerceptor[] = [
    new DatePerceptor(),
]

function App() {
    const [photos, setPhotos] = useState(photoSet);
    const [index, setIndex] = useState(-1);

    const [singleOn, setSingleOn] = useState(false);

    return (
        <div className="App">
            {!singleOn && <header className="App-header">
                <TopBar photos={photos} index={index}
                />
            </header>}
            <GridView
                photos={photos} index={index} setIndex={setIndex}
                openSingle={setSingleOn}
            />
            <SingleView
                photos={photos} index={index} setIndex={setIndex}
                isSingleOpened={singleOn} openSingle={setSingleOn}
            />
        </div>
    );
}

export default App;
