package config

import "path/filepath"

// Database drivers (Database.Driver)
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// Database: the `database` section. GORM hides the driver, so the rest of the
// model does not depend on it.
type Database struct {
	Driver string `yaml:"driver"`
	// sqlite: the database file; server drivers: the database name
	Name string `yaml:"name"`
	// Server drivers only; sqlite ignores them
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`
}

// resolve: sqlite by default; sqlite's Name is a file, inside dataDir unless absolute
// (a server driver's is a database name)
func (d *Database) resolve(dataDir string) {
	if d.Driver == "" {
		d.Driver = DriverSQLite
	}
	if d.Driver != DriverSQLite {
		return
	}
	if d.Name == "" {
		d.Name = defaultDBName
	}
	if !filepath.IsAbs(d.Name) {
		d.Name = filepath.Join(dataDir, d.Name)
	}
}
