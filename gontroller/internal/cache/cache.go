// Package cache: where an item's files live in the data's cache — a tree, not one
// directory of every GUID: two levels by the GUID's first characters, as git's
// objects and Immich's thumbnails. GUIDs are random, so the items spread evenly: at
// most 256 directories a level, about 15 items a leaf at a million.
package cache

import (
	"path/filepath"
	"strings"

	"github.com/eggs-gd/perceplib/api"
)

// ItemDir: an item's directory under a part of the cache ("r", "previews"), relative
// to the cache: <part>/<ab>/<cd>/<guid> for a GUID abcd… (lower case: one tree on a
// case-insensitive file system too)
func ItemDir(part string, guid api.GUID) string {
	shard := strings.ToLower(guid.String())
	if len(shard) < 4 {
		return filepath.Join(part, "_", guid.String())
	}
	return filepath.Join(part, shard[0:2], shard[2:4], guid.String())
}
