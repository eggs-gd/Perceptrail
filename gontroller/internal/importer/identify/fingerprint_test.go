package identify

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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
