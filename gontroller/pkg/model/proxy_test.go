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

func TestConfigureDrivers(t *testing.T) {
	if err := Configure(DBConfig{Driver: DriverSQLite, Name: "x.db"}); err != nil {
		t.Errorf("sqlite: %v", err)
	}
	if err := Configure(DBConfig{Driver: DriverPostgres}); err == nil {
		t.Error("postgres: want a not-implemented error")
	}
	if err := Configure(DBConfig{Driver: "mysql"}); err == nil {
		t.Error("mysql: want an unknown-driver error")
	}
}
