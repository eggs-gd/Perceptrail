import type {SvItem} from "$lib/server/server.api";

export async function load() {
    const initialData = await fetchDataFromDatabase();
    return {
        initialData,
    };
}

async function fetchDataFromDatabase() {
    const sv = `${process.env.SVEBAPP_SERVER_HOST}:${process.env.SVEBAPP_SERVER_PORT}`
    try {
        const data = await fetch(`http://${sv}/items`);
        const jdata = await data.json();
        return prepareItems(jdata)
    } catch (err) {
        console.log(err);
    }

    return {};
}

function prepareItems(input: {}[]): SvItem[] {
    const out: SvItem[] = []
    input.map((itm) => {
        out.push(itm as SvItem);
    })
    return out;
}
