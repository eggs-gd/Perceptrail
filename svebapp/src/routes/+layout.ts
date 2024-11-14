import type {LayoutLoad} from './$types';
import {loadFromServer} from "$lib/stores/proxy";

export const load: LayoutLoad = async ({fetch}) => {
    loadFromServer()
    return {}
}
