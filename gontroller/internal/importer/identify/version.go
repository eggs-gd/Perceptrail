package identify

import (
	l "github.com/eggs-gd/go-zap-decor"
)

// VersionStore: what the cheap stage's version needs — the meta table and the
// item mark that lets the gate run unchanged files through identify again.
type VersionStore interface {
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
	MarkAllRework() (int64, error)
}

// identifyVersion changes whenever identify/exif output for existing unchanged
// files must be rebuilt. 2: size/ratio is oriented by EXIF Orientation/Rotation,
// and previews carry a placeholder color.
const identifyVersion = "2"

const identifyVersionKey = "identify_version"

// reworkOldIdentify runs at start: a new identifyVersion marks every item for one
// cheap-stage pass. The rows keep their updated_at: the client gets the corrected
// item when publish writes it after the pass.
func reworkOldIdentify(db VersionStore, logger *l.Logger) error {
	stored, err := db.GetMeta(identifyVersionKey)
	if err != nil || stored == identifyVersion {
		return err
	}
	n, err := db.MarkAllRework()
	if err != nil {
		return err
	}
	logger.Info("Identify changed: every item is identified once more",
		l.String("from", stored), l.String("to", identifyVersion), l.Int("items", int(n)))
	return db.SetMeta(identifyVersionKey, identifyVersion)
}
