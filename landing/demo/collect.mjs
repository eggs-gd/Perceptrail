// The demo library's manifest: public-domain and CC0 JPEGs from Wikimedia Commons,
// geotagged, around a few cities — every shape (panoramas to portraits), no author
// crowding a city. What it picked is committed (manifest.json): fetch.mjs makes the
// same library from it; this runs again only to pick anew.
//
//   node demo/collect.mjs demo/manifest.json
import {writeFileSync} from 'node:fs';

// Wikimedia asks for a contact in the User-Agent: the project's address
const agent = 'PerceptrailDemo/0.2 (https://github.com/eggs-gd/Perceptrail; demo library)';
const cities = [
    ['Lisbon', 38.7223, -9.1393], ['Paris', 48.8566, 2.3522], ['Kyiv', 50.4501, 30.5234],
    ['Kyoto', 35.0116, 135.7681], ['New York', 40.7128, -74.006], ['Reykjavik', 64.1466, -21.9426],
    ['Cape Town', -33.9249, 18.4241], ['Sydney', -33.8688, 151.2093], ['Rome', 41.9028, 12.4964],
    ['San Francisco', 37.7749, -122.4194], ['Tbilisi', 41.7151, 44.8271],
];
const perCity = 28;
const perAuthor = 3;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
// Commons' HTML as plain text: tags taken out until none is left (one pass can join
// the halves of a tag into a new one)
function text(html) {
    let out = html ?? '';
    for (let prev; prev !== out;) [prev, out] = [out, out.replace(/<[^>]*>/g, '')];
    return out.replace(/[<>]/g, '').trim();
}

// The Commons API, again after a pause while it says it is busy
async function api(params) {
    const url = 'https://commons.wikimedia.org/w/api.php?' + new URLSearchParams({...params, format: 'json'});
    for (let attempt = 1; attempt <= 8; attempt++) {
        const d = await (await fetch(url, {headers: {'User-Agent': agent}})).json();
        if (!d.error) return d;
        await sleep(5000 * attempt);
    }
    throw new Error(`api: ${url}`);
}

// A city's files: imageinfo and coordinates come for part of the pages at a time
async function around(lat, lon) {
    const params = {action: 'query', generator: 'geosearch', ggsnamespace: 6, ggscoord: `${lat}|${lon}`,
        ggsradius: 10000, ggslimit: 400, prop: 'imageinfo|coordinates', colimit: 'max',
        iiprop: 'extmetadata|size|mime|url', iiurlwidth: 2000, iiextmetadatafilter: 'LicenseShortName|DateTimeOriginal|Artist'};
    const pages = new Map();
    for (let more = {}; more;) {
        const d = await api({...params, ...more});
        for (const [id, p] of Object.entries(d.query?.pages ?? {})) {
            const m = pages.get(id) ?? {...p, imageinfo: [], coordinates: []};
            m.imageinfo.push(...(p.imageinfo ?? []));
            m.coordinates.push(...(p.coordinates ?? []));
            pages.set(id, m);
        }
        more = d.continue;
        await sleep(1000);
    }
    return [...pages.values()];
}

const shapeOf = (r) => (r > 2 ? 'pano' : r > 1.5 ? 'wide' : r > 1.1 ? 'land' : r > 0.9 ? 'square' : 'tall');

const manifest = [];
for (const [city, lat, lon] of cities) {
    const shapes = {}, authors = {};
    let picked = 0;
    for (const p of await around(lat, lon)) {
        const ii = p.imageinfo[0] ?? {};
        const meta = ii.extmetadata ?? {};
        const license = meta.LicenseShortName?.value ?? '';
        const artist = text(meta.Artist?.value);
        if (ii.mime !== 'image/jpeg' || !/^(Public domain|CC0)/.test(license)) continue;
        if (Math.max(ii.width, ii.height) < 1600 || !p.coordinates.length) continue;
        const shape = shapeOf(ii.width / ii.height);
        if ((shapes[shape] ?? 0) >= perCity / 2 || (authors[artist] ?? 0) >= perAuthor) continue;
        shapes[shape] = (shapes[shape] ?? 0) + 1;
        authors[artist] = (authors[artist] ?? 0) + 1;
        const [c] = p.coordinates;
        manifest.push({city, title: p.title, page: ii.descriptionurl, url: ii.thumburl ?? ii.url, license, artist,
            date: text(meta.DateTimeOriginal?.value), lat: c.lat, lon: c.lon, w: ii.width, h: ii.height, shape});
        if (++picked >= perCity) break;
    }
    console.error(city, picked, shapes);
}
writeFileSync(process.argv[2], JSON.stringify(manifest, null, 1) + '\n');
console.error(manifest.length, 'photos');
