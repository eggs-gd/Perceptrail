package date

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // IANA zones for the coordinates lookup, also in a minimal Docker image

	"github.com/eggs-gd/perceplib/api"
	"github.com/ringsaturn/tzf"
)

// Where the offset of a date came from (ItemDto.DateZone)
const (
	ZoneTag    = "tag"    // the tag itself or its Offset* tag
	ZoneGPS    = "gps"    // local time minus GPS (UTC) time
	ZoneCoords = "coords" // GPS coordinates -> IANA zone
	ZoneFile   = "file"   // file system time, in the zone of the machine that wrote it
	ZoneServer = "server" // nothing else known: the server's zone, assumed
)

// dateInfo is the resolved date of an item: an instant in the local zone of the shot.
type dateInfo struct {
	time   time.Time
	source string // the tag the date came from
	zone   string // Zone*
}

// serverZone is the last resort for a local time without any zone information
var serverZone = time.Local

// resolveDate picks the best date of an item and its zone. Photos carry a local
// wall-clock time (DateTimeOriginal) and maybe its offset; QuickTime videos carry
// UTC (CreateDate) and maybe a zoned CreationDate; everything has FileModifyDate.
func resolveDate(exif api.ExifProvider) (dateInfo, bool) {
	if strings.HasPrefix(exif.GetExif("MIMEType"), "video/") {
		if d, ok := zonedTag(exif, "CreationDate"); ok { // Keys (Apple): local time + zone
			return d, true
		}
		if t, ok := parseWall(exif.GetExif("CreateDate")); ok { // QuickTime: UTC by spec
			return withZoneOf(exif, t, "CreateDate"), true
		}
	} else {
		wallTags := []struct{ tag, subsec, offset string }{
			{"DateTimeOriginal", "SubSecTimeOriginal", "OffsetTimeOriginal"},
			{"CreateDate", "SubSecTimeDigitized", "OffsetTimeDigitized"},
		}
		for _, w := range wallTags {
			if d, ok := localTag(exif, w.tag, w.subsec, w.offset); ok {
				return d, true
			}
		}
		if t, ok := gpsTime(exif); ok {
			return withZoneOf(exif, t, "GPSDateTime"), true
		}
		if d, ok := localTag(exif, "ModifyDate", "SubSecTime", "OffsetTime"); ok {
			return d, true
		}
	}

	if d, ok := zonedTag(exif, "FileModifyDate"); ok {
		d.zone = ZoneFile
		return d, true
	}
	return dateInfo{}, false
}

// localTag reads a wall-clock tag and finds its zone: its Offset* tag, the GPS
// time, the GPS coordinates, the server.
func localTag(exif api.ExifProvider, tag, subsecTag, offsetTag string) (dateInfo, bool) {
	raw := exif.GetExif(tag)
	if t, zoned, ok := parseZoned(raw); ok && zoned {
		return dateInfo{t, tag, ZoneTag}, true
	}
	wall, ok := parseWall(raw)
	if !ok {
		return dateInfo{}, false
	}
	wall = wall.Add(subsec(exif.GetExif(subsecTag)))

	if off, ok := parseOffset(exif.GetExif(offsetTag)); ok {
		return dateInfo{inOffset(wall, off), tag, ZoneTag}, true
	}
	if gps, ok := gpsTime(exif); ok {
		// Local time minus UTC time of (nearly) the same moment; GPS fixes lag a
		// little, so round to the finest real-world offset step
		diff := wall.Sub(gps).Round(15 * time.Minute)
		if diff.Abs() <= 14*time.Hour {
			return dateInfo{inOffset(wall, int(diff.Minutes())), tag, ZoneGPS}, true
		}
	}
	if loc := coordsZone(exif); loc != nil {
		return dateInfo{inLocation(wall, loc), tag, ZoneCoords}, true
	}
	return dateInfo{inLocation(wall, serverZone), tag, ZoneServer}, true
}

// zonedTag reads a tag that carries its own zone ("…+03:00")
func zonedTag(exif api.ExifProvider, tag string) (dateInfo, bool) {
	t, zoned, ok := parseZoned(exif.GetExif(tag))
	if !ok || !zoned {
		return dateInfo{}, false
	}
	return dateInfo{t, tag, ZoneTag}, true
}

// withZoneOf puts a UTC instant into the local zone of the shot, if it is known
func withZoneOf(exif api.ExifProvider, utc time.Time, tag string) dateInfo {
	if loc := coordsZone(exif); loc != nil {
		return dateInfo{utc.In(loc), tag, ZoneCoords}
	}
	return dateInfo{utc.In(serverZone), tag, ZoneServer}
}

// inOffset: a wall-clock time (parsed as UTC) at a fixed offset in minutes
func inOffset(wall time.Time, minutes int) time.Time {
	return inLocation(wall, time.FixedZone("", minutes*60))
}

// inLocation: a wall-clock time (parsed as UTC) in loc, DST included
func inLocation(wall time.Time, loc *time.Location) time.Time {
	return time.Date(wall.Year(), wall.Month(), wall.Day(),
		wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), loc)
}

const exifLayout = "2006:01:02 15:04:05"

// parseWall parses "2006:01:02 15:04:05[.frac]" as a wall-clock time (in UTC).
// An all-zero value is not a date.
func parseWall(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) < len(exifLayout) || strings.HasPrefix(s, "0000") {
		return time.Time{}, false
	}
	t, err := time.Parse(exifLayout, s[:len(exifLayout)])
	if err != nil {
		return time.Time{}, false
	}
	if rest := s[len(exifLayout):]; strings.HasPrefix(rest, ".") {
		t = t.Add(subsec(rest[1:]))
	}
	return t, true
}

// parseZoned parses a date that may end with a zone ("+03:00", "Z"). zoned tells
// whether it had one.
func parseZoned(s string) (t time.Time, zoned bool, ok bool) {
	wall, ok := parseWall(s)
	if !ok {
		return time.Time{}, false, false
	}
	rest := strings.TrimLeft(strings.TrimSpace(s)[len(exifLayout):], ".0123456789")
	if rest == "Z" {
		return wall, true, true
	}
	if off, ok := parseOffset(rest); ok {
		return inOffset(wall, off), true, true
	}
	return wall, false, true
}

// parseOffset parses "+02:00" / "-05:30" into minutes
func parseOffset(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 6 || (s[0] != '+' && s[0] != '-') || s[3] != ':' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[1:3])
	m, err2 := strconv.Atoi(s[4:6])
	if err1 != nil || err2 != nil || h > 14 || m > 59 {
		return 0, false
	}
	minutes := h*60 + m
	if s[0] == '-' {
		minutes = -minutes
	}
	return minutes, true
}

// subsec turns SubSecTime digits ("47" = .47 s) into a duration
func subsec(s string) time.Duration {
	s = digits(strings.TrimSpace(s))
	if s == "" {
		return 0
	}
	if len(s) > 9 {
		s = s[:9]
	}
	n, err := strconv.Atoi(s + strings.Repeat("0", 9-len(s)))
	if err != nil {
		return 0
	}
	return time.Duration(n)
}

// digits returns the leading decimal digits of s
func digits(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// gpsTime is the GPS (UTC) time: the composite GPSDateTime or the date and time stamps
func gpsTime(exif api.ExifProvider) (time.Time, bool) {
	if t, _, ok := parseZoned(exif.GetExif("GPSDateTime")); ok {
		return t.UTC(), true
	}
	date := strings.TrimSpace(exif.GetExif("GPSDateStamp"))
	clock := strings.TrimSpace(exif.GetExif("GPSTimeStamp"))
	if date == "" || clock == "" {
		return time.Time{}, false
	}
	t, ok := parseWall(date + " " + clock)
	return t, ok
}

// Coordinates as exiftool prints them: 50 deg 27' 12.34" N
var dmsRe = regexp.MustCompile(`(\d+(?:\.\d+)?) deg (\d+(?:\.\d+)?)' (\d+(?:\.\d+)?)"(?: ([NSEW]))?`)

// coords returns latitude and longitude from GPSLatitude/GPSLongitude or the
// QuickTime GPSCoordinates
func coords(exif api.ExifProvider) (lat, lng float64, ok bool) {
	if c := exif.GetExif("GPSCoordinates"); c != "" {
		m := dmsRe.FindAllStringSubmatch(c, 2)
		if len(m) == 2 {
			return dms(m[0], exif.GetExif("GPSLatitudeRef")), dms(m[1], exif.GetExif("GPSLongitudeRef")), true
		}
	}
	la := dmsRe.FindStringSubmatch(exif.GetExif("GPSLatitude"))
	lo := dmsRe.FindStringSubmatch(exif.GetExif("GPSLongitude"))
	if la == nil || lo == nil {
		return 0, 0, false
	}
	return dms(la, exif.GetExif("GPSLatitudeRef")), dms(lo, exif.GetExif("GPSLongitudeRef")), true
}

// dms converts a dmsRe match to degrees; the direction comes from the value or the Ref tag
func dms(m []string, ref string) float64 {
	d, _ := strconv.ParseFloat(m[1], 64)
	min, _ := strconv.ParseFloat(m[2], 64)
	sec, _ := strconv.ParseFloat(m[3], 64)
	v := d + min/60 + sec/3600
	dir := m[4]
	if dir == "" && ref != "" {
		dir = ref[:1] // "North", "South", "East", "West"
	}
	if dir == "S" || dir == "W" {
		v = -v
	}
	return v
}

var (
	finderOnce sync.Once
	finder     tzf.F
)

// coordsZone is the IANA zone at the GPS coordinates, nil if unknown. The zone
// dictionary is loaded on first use.
func coordsZone(exif api.ExifProvider) *time.Location {
	lat, lng, ok := coords(exif)
	if !ok || math.Abs(lat) > 90 || math.Abs(lng) > 180 || (lat == 0 && lng == 0) {
		return nil
	}
	finderOnce.Do(func() {
		f, err := tzf.NewDefaultFinder()
		if err == nil {
			finder = f
		}
	})
	if finder == nil {
		return nil
	}
	name := finder.GetTimezoneName(lng, lat)
	if name == "" {
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	return loc
}
