package model

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"perceptrail/gontroller/internal/config"

	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/go-zap-decor/tree"
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

// The driver: sqlite opens; postgres is not implemented yet; an unknown one fails
func TestOpenDrivers(t *testing.T) {
	logger := l.NewLogger(l.ErrorLevel, &tree.Decorator{})
	db, err := Open(database{config.Database{Driver: config.DriverSQLite, Name: filepath.Join(t.TempDir(), "x.db")}}, logger)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	db.Close()
	if _, err := Open(database{config.Database{Driver: config.DriverPostgres}}, logger); err == nil {
		t.Error("postgres: want a not-implemented error")
	}
	if _, err := Open(database{config.Database{Driver: "mysql"}}, logger); err == nil {
		t.Error("mysql: want an unknown-driver error")
	}
}

// database: a Config of one database
type database struct{ db config.Database }

func (d database) Database() config.Database { return d.db }
