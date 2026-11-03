// The demo library from its manifest: each photo's 2000 px rendition from Wikimedia
// Commons, its date and place written into its EXIF — the Commons date when it has
// one, else its city's first dated day (one trip); the Commons coordinates. Files
// already there are not fetched again. exiftool -m: a minor fault of a file (a cut
// maker IFD) does not stop it.
//
//   node demo/fetch.mjs demo/manifest.json <library dir>
import {execFileSync} from 'node:child_process';
import {existsSync, mkdirSync, readFileSync, writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {exifDate, fileOf} from './names.mjs';

const agent = 'PerceptrailDemo/0.2 (https://github.com/eggs-gd/Perceptrail; demo library)';
const [manifestPath, dir] = process.argv.slice(2);
const photos = JSON.parse(readFileSync(manifestPath, 'utf8'));
mkdirSync(dir, {recursive: true});

const trips = {};
for (const p of photos) trips[p.city] ??= exifDate(p.date) ?? undefined;

for (const p of photos) {
    const path = join(dir, fileOf(p));
    if (!existsSync(path)) {
        const res = await fetch(p.url, {headers: {'User-Agent': agent}});
        if (!res.ok) throw new Error(`${p.url}: ${res.status}`);
        writeFileSync(path, Buffer.from(await res.arrayBuffer()));
        await new Promise((r) => setTimeout(r, 1000));
    }
    const date = exifDate(p.date) ?? trips[p.city] ?? '2018:06:01 12:00:00';
    execFileSync('exiftool', ['-q', '-m', '-overwrite_original', '-n', `-DateTimeOriginal=${date}`,
        `-GPSLatitude=${Math.abs(p.lat)}`, `-GPSLatitudeRef=${p.lat >= 0 ? 'N' : 'S'}`,
        `-GPSLongitude=${Math.abs(p.lon)}`, `-GPSLongitudeRef=${p.lon >= 0 ? 'E' : 'W'}`, '-Orientation=1', path]);
}
console.error(photos.length, 'photos in', dir);
