import {describe, expect, it} from 'vitest';
import {readFileSync} from 'node:fs';
import {exifDate, fileOf} from './names.mjs';

const manifest = JSON.parse(readFileSync(new URL('./manifest.json', import.meta.url), 'utf8'));

describe('fileOf', () => {
    it('gives every photo of the manifest a file of its own', () => {
        expect(new Set(manifest.map(fileOf)).size).toBe(manifest.length);
    });
    it('keeps a name of other scripts apart', () => {
        expect(fileOf({title: 'File:京都市役所 分庁舎.jpg'})).not.toBe(fileOf({title: 'File:辨天堂（京都 天性寺）.jpg'}));
    });
});

describe('exifDate', () => {
    it.each([
        ['2017-05-04 12:34', '2017:05:04 12:34:00'],
        ['2017-05-04T12:34:56', '2017:05:04 12:34:56'],
        ['Taken on 2 February 2006, 13:36 (according to Exif data)', '2006:02:02 13:36:00'],
        ['7 November 2011, 16:39:54 (UTC)', '2011:11:07 16:39:54'],
        ['24 June 2026 12:32:37', '2026:06:24 12:32:37'],
        ['Taken on 13 December 2016', '2016:12:13 12:00:00'],
        ['24 September 2015 (original upload date)Taken on 18 July 2015, 06:25 (according to Exif data)', '2015:07:18 06:25:00'],
        ['1934-10', '1934:10:01 12:00:00'],
        ['1952', '1952:01:01 12:00:00'],
    ])('%s', (date, want) => {
        expect(exifDate(date)).toBe(want);
    });
    it('has none for no date', () => {
        expect(exifDate('')).toBeNull();
        expect(exifDate('unknown')).toBeNull();
    });
    it('reads every dated photo of the manifest', () => {
        expect(manifest.filter((p) => p.date && !exifDate(p.date)).map((p) => p.date)).toEqual([]);
    });
});
