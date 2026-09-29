package model

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// A data_dir with URI-significant characters must open exactly that file
func TestSqliteDSNEscapesPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a#b?c%20 d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite3", sqliteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE t (x)"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("database not created at %q: %v", path, err)
	}
}
