package identify

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"

	"perceptrail/gontroller/internal/model/dto"

	l "github.com/eggs-gd/go-zap-decor"
)

// HashStore: what the fingerprint's version needs — the meta table (the version
// the items were fingerprinted with) and the items' fingerprints it clears
type HashStore interface {
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
	ClearHashes() (int64, error)
}

// The fingerprint changes: a new hashVersion makes every item forget its old one
// (the gate sends each group once more, validate gives it the new one — the same
// path keeps its GUID). Without it an untouched file keeps the old fingerprint and
// a later move of it looks like a new item.
const hashVersion = "2" // 2: from the file's bytes (1: from its exif)

const hashVersionKey = "hash_version"

// fingerprintSample: the bytes read from each end of the main file
const fingerprintSample = 64 << 10

// fingerprint: the main file's identity across paths (moved, duplicate, changed),
// from its bytes: the size and the first and last 64 KB. No exiftool: it does not
// depend on which tags are read.
func fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()
	h := sha256.New()
	if err := binary.Write(h, binary.LittleEndian, size); err != nil {
		return "", err
	}
	if size <= 2*fingerprintSample {
		_, err = io.Copy(h, f)
	} else {
		if _, err = io.CopyN(h, f, fingerprintSample); err == nil {
			_, err = io.Copy(h, io.NewSectionReader(f, size-fingerprintSample, fingerprintSample))
		}
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// pixelsHash: the fingerprint of the files the item's pixels are read from (render
// reads the main file, the preview when it is big enough, the motion) — the main
// file's, and each other local file of the group shown as the preview or played as
// the motion; only the main file: its own fingerprint. An extracted preview is not
// one of them: it is the main file's.
func pixelsHash(d *draft) (string, error) {
	var others []string
	for _, f := range d.Files[1:] {
		if !f.Remote() && (f.Path == d.Item.PreviewPath || f.Role == dto.RoleMotion) {
			others = append(others, f.Path)
		}
	}
	if len(others) == 0 {
		return d.Hash, nil
	}
	h := sha256.New()
	h.Write([]byte(d.Hash))
	for _, path := range others {
		sum, err := fingerprint(path)
		if err != nil {
			return "", err
		}
		h.Write([]byte(sum))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// forgetOldHashes runs at start: a new hashVersion clears every item's fingerprint
func forgetOldHashes(db HashStore, logger *l.Logger) error {
	stored, err := db.GetMeta(hashVersionKey)
	if err != nil || stored == hashVersion {
		return err
	}
	n, err := db.ClearHashes()
	if err != nil {
		return err
	}
	if n > 0 { // a new database has nothing to do again: nothing to say
		logger.Info("Fingerprint changed: every item is identified once more",
			l.String("from", stored), l.String("to", hashVersion), l.Int("items", int(n)))
	}
	return db.SetMeta(hashVersionKey, hashVersion)
}
