package apple

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/eggs-gd/perceplib/api"
)

// The library's DB is the truth for an asset: the date, the place and the size
// the user sees (and may have corrected) in Photos — the file keeps its old EXIF,
// and a cloud-only asset has no original to read at all. The DB values go into
// the group as a metadata record with exiftool's tag names, so the core plugins
// read them unchanged; RawItem.GetExif looks there first.

// Core Data timestamps count from 2001-01-01 UTC
var coreDataEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

type assetMeta struct {
	created       sql.NullFloat64 // ZASSET.ZDATECREATED
	width, height sql.NullInt64   // ZASSET.ZWIDTH/ZHEIGHT: already oriented
	lat, lon      sql.NullFloat64 // ZASSET.ZLATITUDE/ZLONGITUDE: -180 = none
	tzOffset      sql.NullInt64   // ZADDITIONALASSETATTRIBUTES.ZTIMEZONEOFFSET, seconds east
	duration      sql.NullFloat64 // ZASSET.ZDURATION, seconds: a video (a cloud-only one too)
}

// record: the asset's metadata as exiftool would print it (-s2, no groups)
func (m assetMeta) record() api.RawExif {
	r := api.RawExif{}
	if m.created.Valid {
		utc := coreDataEpoch.Add(time.Duration(m.created.Float64 * float64(time.Second)))
		if m.tzOffset.Valid {
			local := utc.Add(time.Duration(m.tzOffset.Int64) * time.Second)
			r["DateTimeOriginal"] = []byte(local.Format("2006:01:02 15:04:05"))
			r["OffsetTimeOriginal"] = []byte(offset(int(m.tzOffset.Int64)))
		} else {
			// The instant without the zone: the date chain then finds the zone
			r["GPSDateTime"] = []byte(utc.Format("2006:01:02 15:04:05") + "Z")
		}
		if ms := utc.Nanosecond() / int(time.Millisecond); ms > 0 {
			r["SubSecTimeOriginal"] = []byte(fmt.Sprintf("%03d", ms))
		}
	}
	if m.width.Int64 > 0 && m.height.Int64 > 0 {
		r["ImageWidth"] = []byte(fmt.Sprint(m.width.Int64))
		r["ImageHeight"] = []byte(fmt.Sprint(m.height.Int64))
		// The DB size is oriented: a file's Orientation/Rotation must not turn it again
		r["Orientation"] = []byte("1")
		r["Rotation"] = []byte("0")
	}
	if m.lat.Valid && m.lon.Valid && m.lat.Float64 != -180 && m.lon.Float64 != -180 {
		r["GPSLatitude"] = []byte(dms(m.lat.Float64, "N", "S"))
		r["GPSLongitude"] = []byte(dms(m.lon.Float64, "E", "W"))
	}
	if m.duration.Float64 > 0 {
		r["Duration"] = []byte(fmt.Sprintf("%.2f s", m.duration.Float64))
	}
	return r
}

func offset(seconds int) string {
	sign := '+'
	if seconds < 0 {
		sign, seconds = '-', -seconds
	}
	return fmt.Sprintf("%c%02d:%02d", sign, seconds/3600, seconds%3600/60)
}

// dms: degrees as exiftool prints them: 50 deg 25' 45.45" N
func dms(v float64, pos, neg string) string {
	dir := pos
	if v < 0 {
		dir, v = neg, -v
	}
	d := math.Floor(v)
	m := math.Floor((v - d) * 60)
	s := ((v-d)*60 - m) * 60
	return fmt.Sprintf("%.0f deg %.0f' %.2f\" %s", d, m, s, dir)
}

// hashRecord: changes when the DB metadata changes (a date corrected in Photos,
// live switched off), so the gate processes a group whose files did not change
func hashRecord(r api.RawExif, kind string) string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write(r[k])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
