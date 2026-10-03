package exif_date

import (
	"testing"
	"time"
)

type tags map[string]string

func (t tags) GetExif(key string) string { return t[key] }

// Tag sets as exiftool prints them (-s2, no groups); values from real files where noted
func TestResolveDate(t *testing.T) {
	kyivServer := time.FixedZone("server", 3*3600)
	serverZone = kyivServer
	t.Cleanup(func() { serverZone = time.Local })

	cases := []struct {
		name   string
		tags   tags
		want   string // RFC3339 in the local zone of the shot
		source string
		zone   string
	}{
		{
			name: "iPhone HEIC: local time + its offset (real file)",
			tags: tags{"MIMEType": "image/heic", "DateTimeOriginal": "2025:01:19 15:16:09",
				"OffsetTimeOriginal": "+02:00", "GPSDateStamp": "2025:01:19", "GPSTimeStamp": "13:15:59.47"},
			want: "2025-01-19T15:16:09+02:00", source: "DateTimeOriginal", zone: ZoneTag,
		},
		{
			name: "sub-seconds kept",
			tags: tags{"DateTimeOriginal": "2025:01:19 15:16:09", "SubSecTimeOriginal": "47", "OffsetTimeOriginal": "+02:00"},
			want: "2025-01-19T15:16:09.47+02:00", source: "DateTimeOriginal", zone: ZoneTag,
		},
		{
			name: "camera without offset, GPS time: offset from the difference",
			tags: tags{"DateTimeOriginal": "2024:07:10 18:05:00", "GPSDateStamp": "2024:07:10", "GPSTimeStamp": "15:04:31"},
			want: "2024-07-10T18:05:00+03:00", source: "DateTimeOriginal", zone: ZoneGPS,
		},
		{
			name: "no offset, no GPS time, coordinates: Kyiv in summer (DST)",
			tags: tags{"DateTimeOriginal": "2024:07:10 18:05:00",
				"GPSLatitude": "50.45", "GPSLongitude": "30.516667"},
			want: "2024-07-10T18:05:00+03:00", source: "DateTimeOriginal", zone: ZoneCoords,
		},
		{
			name: "signed coordinates (west negative): New York in winter",
			tags: tags{"DateTimeOriginal": "2024:01:10 09:00:00",
				"GPSLatitude":  "40.712778",
				"GPSLongitude": "-74.006111"},
			want: "2024-01-10T09:00:00-05:00", source: "DateTimeOriginal", zone: ZoneCoords,
		},
		{
			name: "nothing but a local time: the server zone, assumed",
			tags: tags{"DateTimeOriginal": "2023:02:21 17:07:36"},
			want: "2023-02-21T17:07:36+03:00", source: "DateTimeOriginal", zone: ZoneServer,
		},
		{
			name: "zero DateTimeOriginal is skipped",
			tags: tags{"DateTimeOriginal": "0000:00:00 00:00:00", "CreateDate": "2022:05:01 10:00:00", "OffsetTimeDigitized": "+01:00"},
			want: "2022-05-01T10:00:00+01:00", source: "CreateDate", zone: ZoneTag,
		},
		{
			name: "QuickTime video: zoned CreationDate (real file)",
			tags: tags{"MIMEType": "video/quicktime", "CreateDate": "2024:09:23 11:29:03",
				"CreationDate": "2024:09:23 14:29:03+03:00"},
			want: "2024-09-23T14:29:03+03:00", source: "CreationDate", zone: ZoneTag,
		},
		{
			name: "video without CreationDate: CreateDate is UTC",
			tags: tags{"MIMEType": "video/mp4", "CreateDate": "2024:09:23 11:29:03",
				"GPSCoordinates": "50.45 30.516667 150"},
			want: "2024-09-23T14:29:03+03:00", source: "CreateDate", zone: ZoneCoords,
		},
		{
			name: "only GPS time: the instant, zone from coordinates",
			tags: tags{"GPSDateStamp": "2025:01:19", "GPSTimeStamp": "13:15:59",
				"GPSLatitude": "48.856667", "GPSLongitude": "2.352222"},
			want: "2025-01-19T14:15:59+01:00", source: "GPSDateTime", zone: ZoneCoords,
		},
		{
			name: "PNG without EXIF: FileModifyDate with its zone (was never parsed)",
			tags: tags{"MIMEType": "image/png", "FileModifyDate": "2026:09:24 21:29:46+03:00"},
			want: "2026-09-24T21:29:46+03:00", source: "FileModifyDate", zone: ZoneFile,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, ok := resolveDate(c.tags)
			if !ok {
				t.Fatal("no date")
			}
			if got := d.time.Format(time.RFC3339Nano); got != c.want {
				t.Errorf("time %s, want %s", got, c.want)
			}
			if d.source != c.source || d.zone != c.zone {
				t.Errorf("source/zone %s/%s, want %s/%s", d.source, d.zone, c.source, c.zone)
			}
		})
	}

	if _, ok := resolveDate(tags{"MIMEType": "image/jpeg"}); ok {
		t.Error("a date out of nothing")
	}
}
