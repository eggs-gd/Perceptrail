package identify

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/pkg/model/dto"

	l "github.com/eggs-gd/perceplib/logger"
	"github.com/eggs-gd/perceplib/logger/decorators"
)

// The same bytes anywhere: the same fingerprint; another byte at either end or
// another size: another one
func TestFingerprint(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, content []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	fp := func(p string) string {
		t.Helper()
		h, err := fingerprint(p)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	big := bytes.Repeat([]byte("x"), 3*fingerprintSample)
	tail := append(bytes.Clone(big[:len(big)-1]), 'y')
	head := append([]byte("y"), big[1:]...)

	a, moved := fp(write("a.jpg", big)), fp(write("sub-a.jpg", big))
	if a != moved {
		t.Error("the same bytes, another fingerprint")
	}
	for name, content := range map[string][]byte{"tail": tail, "head": head, "size": big[:len(big)-1], "small": []byte("small")} {
		if fp(write(name, content)) == a {
			t.Errorf("%s: the same fingerprint as another file", name)
		}
	}
}

// A new hashVersion clears every item's fingerprint once (the gate then sends their
// groups again); the same version clears nothing
func TestForgetOldHashes(t *testing.T) {
	logger := l.NewLogger(l.ErrorLevel, &decorators.GontrollerDecorator{})
	f, err := testDB.CreateFile(dto.ItemEntry{Path: filepath.Join(t.TempDir(), "old.jpg"), Name: "old.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := testDB.CreateItem(f)
	if err != nil {
		t.Fatal(err)
	}
	item.HashShort = "exif-hash"
	if _, err := testDB.UpdateItem(item); err != nil {
		t.Fatal(err)
	}
	if err := testDB.SetMeta(hashVersionKey, "1"); err != nil {
		t.Fatal(err)
	}

	if err := forgetOldHashes(testDB, logger); err != nil {
		t.Fatal(err)
	}
	if got, _ := testDB.GetItemByGuid(item.Guid); got.HashShort != "" {
		t.Errorf("fingerprint kept: %q", got.HashShort)
	}
	if v, _ := testDB.GetMeta(hashVersionKey); v != hashVersion {
		t.Errorf("version %q, want %q", v, hashVersion)
	}

	item.HashShort = "new-hash"
	if _, err := testDB.UpdateItem(item); err != nil {
		t.Fatal(err)
	}
	if err := forgetOldHashes(testDB, logger); err != nil {
		t.Fatal(err)
	}
	if got, _ := testDB.GetItemByGuid(item.Guid); got.HashShort != "new-hash" {
		t.Errorf("the same version cleared it: %q", got.HashShort)
	}
}
