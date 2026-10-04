package config

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
