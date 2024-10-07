import React, {useState} from 'react';
import './App.css';
import {photos} from './misc/photos';

import Grid from "./gallery/Grid";
import Focused from "./gallery/Focused";

function App() {
    const [index, setIndex] = useState(-1);
    return (
        <div className="App">
            <header className="App-header"/>
            <div>
                <Grid photos={photos} index={index} setIndex={setIndex}/>
                <Focused photos={photos} index={index} setIndex={setIndex}/>
            </div>
        </div>
    );
}

export default App;
