package model

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eggs-gd/perceplib/api"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// PerceptorStore keeps one perceptor's values per item (api.Store[T] on the plugin
// side). SQLite: a file of its own, data_dir/perceptors/<store>.db — an ML perceptor
// writing does not block the import (one writer per file), and deleting the file
// resets its data. Rows are keyed by the item's GUID; has = 0 records "processed,
// nothing found" (an item without GPS), so it is not processed again on every walk.
// A changed schema (version or fields) drops the values: the items are processed
// again (the files gate asks Has).
type PerceptorStore struct {
	schema api.Schema
	db     *gorm.DB
}

const valuesTable = "item_values"

func OpenPerceptorStore(driver, dir string, s api.Schema) (*PerceptorStore, error) {
	if driver != DriverSQLite {
		return nil, fmt.Errorf("perceptor storage on %s: not implemented yet", driver)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(sqliteDSN(filepath.Join(dir, s.Store+".db"))),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return nil, err
	}
	if err := sqliteDriver.tune(db); err != nil {
		return nil, err
	}
	st := &PerceptorStore{schema: s, db: db}
	return st, st.migrate()
}

func (s *PerceptorStore) Name() string { return s.schema.Store }

// migrate: the stored schema differs from the declared one → the values go
func (s *PerceptorStore) migrate() error {
	desc, _ := json.Marshal(s.schema)
	if err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_info (id INTEGER PRIMARY KEY CHECK (id = 1), schema TEXT NOT NULL)`).Error; err != nil {
		return err
	}
	var stored string // none yet: a new store
	err := s.db.Raw(`SELECT schema FROM schema_info WHERE id = 1`).Row().Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if stored == string(desc) {
		return nil
	}
	cols := []string{`guid TEXT PRIMARY KEY`, `has INTEGER NOT NULL`}
	for _, f := range s.schema.Fields {
		cols = append(cols, fmt.Sprintf(`%q %s`, f.Name, sqlType(f.Kind)))
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DROP TABLE IF EXISTS ` + valuesTable).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TABLE ` + valuesTable + ` (` + strings.Join(cols, ", ") + `)`).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT OR REPLACE INTO schema_info (id, schema) VALUES (1, ?)`, string(desc)).Error
	})
}

func sqlType(k api.Kind) string {
	switch k {
	case api.KindFloat:
		return "REAL"
	case api.KindInt, api.KindBool, api.KindTime:
		return "INTEGER"
	case api.KindVector:
		return "BLOB"
	}
	return "TEXT"
}

// Has: the item was processed by this perceptor (with or without a value)
func (s *PerceptorStore) Has(guid string) (bool, error) {
	var n int64
	err := s.db.Raw(`SELECT COUNT(*) FROM `+valuesTable+` WHERE guid = ?`, guid).Row().Scan(&n)
	return n > 0, err
}

// Save: the item's values; nil records "processed, nothing found"
func (s *PerceptorStore) Save(guid string, v api.Values) error {
	names := []string{"guid", "has"}
	args := []any{guid, v != nil}
	for _, f := range s.schema.Fields {
		names = append(names, fmt.Sprintf("%q", f.Name))
		args = append(args, encode(f, v[f.Name]))
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")
	return s.db.Exec(`INSERT OR REPLACE INTO `+valuesTable+` (`+strings.Join(names, ", ")+`) VALUES (`+marks+`)`, args...).Error
}

// Load: the values of these items, those that have some
func (s *PerceptorStore) Load(guids []string) (map[string]api.Values, error) {
	out := map[string]api.Values{}
	cols := []string{"guid"}
	for _, f := range s.schema.Fields {
		cols = append(cols, fmt.Sprintf("%q", f.Name))
	}
	const chunk = 500 // under SQLite's variable limit
	for start := 0; start < len(guids); start += chunk {
		page := guids[start:min(start+chunk, len(guids))]
		rows, err := s.db.Raw(`SELECT `+strings.Join(cols, ", ")+` FROM `+valuesTable+` WHERE has = 1 AND guid IN ?`, page).Rows()
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var guid string
			raw := make([]any, len(s.schema.Fields))
			dest := []any{&guid}
			for i := range raw {
				dest = append(dest, &raw[i])
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return nil, err
			}
			v := api.Values{}
			for i, f := range s.schema.Fields {
				v[f.Name] = decode(f, raw[i])
			}
			out[guid] = v
		}
		// An error mid-way ends Next() too: not a partial answer passed off as whole
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Guids: every item this perceptor has processed (a row, with or without a value)
func (s *PerceptorStore) Guids() ([]string, error) {
	var guids []string
	err := s.db.Raw(`SELECT guid FROM ` + valuesTable).Scan(&guids).Error
	return guids, err
}

// Prune: drops the rows of items that are gone (keep says which stay)
func (s *PerceptorStore) Prune(keep func(guid string) bool) (int, error) {
	guids, err := s.Guids()
	if err != nil {
		return 0, err
	}
	var gone []string
	for _, g := range guids {
		if !keep(g) {
			gone = append(gone, g)
		}
	}
	for start := 0; start < len(gone); start += 500 {
		page := gone[start:min(start+500, len(gone))]
		if err := s.db.Exec(`DELETE FROM `+valuesTable+` WHERE guid IN ?`, page).Error; err != nil {
			return 0, err
		}
	}
	return len(gone), nil
}

func (s *PerceptorStore) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// encode: a Go value of the field's kind as SQLite stores it
func encode(f api.Field, v any) any {
	if v == nil {
		return nil
	}
	switch f.Kind {
	case api.KindTime:
		if t, ok := v.(time.Time); ok {
			return t.UnixNano()
		}
	case api.KindBool:
		if b, ok := v.(bool); ok && b {
			return 1
		}
		return 0
	case api.KindVector:
		if vec, ok := v.([]float32); ok {
			buf := make([]byte, 4*len(vec))
			for i, x := range vec {
				binary.LittleEndian.PutUint32(buf[4*i:], math.Float32bits(x))
			}
			return buf
		}
	}
	return v
}

// decode: back to the Go type the field's kind means
func decode(f api.Field, v any) any {
	if v == nil {
		return nil
	}
	switch f.Kind {
	case api.KindFloat:
		if x, ok := v.(float64); ok {
			return x
		}
		if x, ok := v.(int64); ok {
			return float64(x)
		}
	case api.KindInt:
		if x, ok := v.(int64); ok {
			return x
		}
	case api.KindBool:
		if x, ok := v.(int64); ok {
			return x != 0
		}
	case api.KindTime:
		if x, ok := v.(int64); ok {
			return time.Unix(0, x).UTC()
		}
	case api.KindVector:
		if b, ok := v.([]byte); ok {
			vec := make([]float32, len(b)/4)
			for i := range vec {
				vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
			}
			return vec
		}
	case api.KindText:
		switch x := v.(type) {
		case string:
			return x
		case []byte:
			return string(x)
		}
	}
	return nil
}
