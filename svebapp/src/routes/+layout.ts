import type {LayoutLoad} from './$types';
import {loadFromServer, updateLayout} from "$lib/stores/proxy";
//import {screenWidth, rowHeight} from "$lib/stores/stores";

export const load: LayoutLoad = async ({fetch}) => {
    loadFromServer()
    //updateLayout($screenWidth, $rowHeight)
    return {}
}
