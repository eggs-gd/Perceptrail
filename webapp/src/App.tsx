import React, {useState} from 'react';
import './App.css';
import {photos} from './misc/photos';

import GridView from "./gallery/GridView";
import SingleView from "./gallery/SingleView";
import TopBar from "./gallery/TopBar";
import DetailsView from "./gallery/DetailsView";
import FocusButtons from "./gallery/FocusButtons";

function App() {
    const [index, setIndex] = useState(-1);
    return (
        <div className="App">
            <header className="App-header"/>
            <div>
                <TopBar photos={photos} index={index} setIndex={setIndex}/>
                <GridView photos={photos} index={index} setIndex={setIndex}/>
            </div>
            <div>
                <SingleView photos={photos} index={index} setIndex={setIndex}/>
                <DetailsView photos={photos} index={index} setIndex={setIndex}/>
                <FocusButtons photos={photos} index={index} setIndex={setIndex}/>
            </div>
        </div>
    );
}

export default App;
