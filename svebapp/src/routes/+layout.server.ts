import * as fs from "node:fs/promises";

export async function load() {

    try {
        const data = await fs.readFile('%sveltekit.assets%/photos.json', { encoding: 'utf8' });
        return await JSON.parse(data);
    } catch (err) {
        console.log(err);
    }

    return {};
}
