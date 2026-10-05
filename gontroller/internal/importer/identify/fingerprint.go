package identify

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"

	l "github.com/eggs-gd/go-zap-decor"
)

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

// HashStore: what the fingerprint's version needs — the meta table (the version
// the items were fingerprinted with) and the items' fingerprints it clears
type HashStore interface {
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
	ClearHashes() (int64, error)
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
	logger.Info("Fingerprint changed: every item is identified once more",
		l.String("from", stored), l.String("to", hashVersion), l.Int("items", int(n)))
	return db.SetMeta(hashVersionKey, hashVersion)
}
