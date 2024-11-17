import type {LayoutLoad} from './$types';
import {loadFromServer, updateLayout} from "$lib/workers";
//import {screenWidth, rowHeight} from "$lib/stores/stores";

export const load: LayoutLoad = async ({fetch}) => {
    loadFromServer()
    //updateLayout($screenWidth, $rowHeight)
    return {}
}
