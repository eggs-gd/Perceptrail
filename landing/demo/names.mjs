// What fetch.mjs makes of a manifest entry: its file's name, its EXIF date
import {createHash} from 'node:crypto';

const months = ['january', 'february', 'march', 'april', 'may', 'june', 'july', 'august', 'september', 'october', 'november', 'december'];

// The file of a photo: its title as a name to read, and a hash of the whole title —
// titles in other scripts come to one name to read (Kyiv's and Kyoto's to "jpg")
export function fileOf(photo) {
    const name = photo.title.toLowerCase().replace(/^file:/, '').replace(/[^a-z0-9]+/g, '-').slice(0, 80).replace(/^-|-$/g, '');
    return `${name}-${createHash('sha1').update(photo.title).digest('hex').slice(0, 8)}.jpg`;
}

// EXIF's date ("2017:05:04 12:00:00") of Commons', in the forms it comes in —
// "2017-05-04 12:00", "4 May 2017, 12:00:30 (UTC)", "Taken on 4 May 2017", "2017-05",
// "1952" — if it has one. An upload date before "Taken on": the taken one.
export function exifDate(date) {
    const text = (date ?? '').split('Taken on').at(-1);
    const iso = /(\d{4})-(\d\d)(?:-(\d\d))?(?:[ T](\d\d):(\d\d)(?::(\d\d))?)?/.exec(text);
    if (iso) return stamp(iso[1], iso[2], iso[3], iso[4], iso[5], iso[6]);
    const long = /(\d{1,2}) ([A-Za-z]+) (\d{4})(?:,? (\d\d):(\d\d)(?::(\d\d))?)?/.exec(text);
    const month = long ? months.indexOf(long[2].toLowerCase()) + 1 : 0;
    if (month) return stamp(long[3], month, long[1], long[4], long[5], long[6]);
    const year = /^\s*(\d{4})\s*$/.exec(text);
    return year ? stamp(year[1]) : null;
}


// The EXIF form; what the date does not say: the first of the month, noon
function stamp(year, month = 1, day = 1, hour = 12, minute = 0, second = 0) {
    return `${year}:${pad(month)}:${pad(day)} ${pad(hour)}:${pad(minute)}:${pad(second)}`;
}

function pad(n) {
    return String(n).padStart(2, '0');
}
