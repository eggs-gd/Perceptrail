import {SVEBAPP_SERVER_HOST, SVEBAPP_SERVER_PORT} from '$env/static/private'

export async function load({fetch}) {
    const sv = `${SVEBAPP_SERVER_HOST}:${SVEBAPP_SERVER_PORT}`
    return {
        streamUrl: `http://${sv}/items`
    };
}
