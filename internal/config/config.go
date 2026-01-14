package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"sslmode"`
}

// SyncConfig holds sync operation configuration
type SyncConfig struct {
	Strategy  string        `mapstructure:"strategy"` // full, incremental
	BatchSize int           `mapstructure:"batch_size"`
	Tables    []TableConfig `mapstructure:"tables"`
}

// TableConfig holds per-table sync configuration
type TableConfig struct {
	Name            string `mapstructure:"name"`
	PrimaryKey      string `mapstructure:"primary_key"`
	TimestampColumn string `mapstructure:"timestamp_column"`
}

// Config holds the complete application configuration
type Config struct {
	Source  DatabaseConfig `mapstructure:"source"`
	Target  DatabaseConfig `mapstructure:"target"`
	Sync    SyncConfig     `mapstructure:"sync"`
	Logging LoggingConfig  `mapstructure:"logging"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// Load loads configuration from viper
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

// ParseDSN parses a database connection string into DatabaseConfig
// Supports formats:
//   - postgres://user:pass@host:port/dbname?sslmode=disable
//   - mysql://user:pass@host:port/dbname
func ParseDSN(dsn string) (*DatabaseConfig, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN format: %w", err)
	}

	cfg := &DatabaseConfig{
		Driver:   u.Scheme,
		Host:     u.Hostname(),
		Database: strings.TrimPrefix(u.Path, "/"),
	}

	// Normalize driver name
	if cfg.Driver == "postgresql" {
		cfg.Driver = "postgres"
	}

	// Parse port
	if u.Port() != "" {
		fmt.Sscanf(u.Port(), "%d", &cfg.Port)
	} else {
		// Default ports
		switch cfg.Driver {
		case "postgres", "postgresql":
			cfg.Port = 5432
			cfg.Driver = "postgres"
		case "mysql":
			cfg.Port = 3306
		}
	}

	// Parse user info
	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}

	// Parse query parameters
	query := u.Query()
	if sslmode := query.Get("sslmode"); sslmode != "" {
		cfg.SSLMode = sslmode
	}

	return cfg, nil
}

// DSN returns the connection string for the database
func (c *DatabaseConfig) DSN() string {
	switch c.Driver {
	case "postgres":
		sslmode := c.SSLMode
		if sslmode == "" {
			sslmode = "disable"
		}
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			c.User, c.Password, c.Host, c.Port, c.Database, sslmode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			c.User, c.Password, c.Host, c.Port, c.Database)
	default:
		return ""
	}
}

// Validate validates the database configuration
func (c *DatabaseConfig) Validate() error {
	if c.Driver == "" {
		return fmt.Errorf("database driver is required")
	}
	if c.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database == "" {
		return fmt.Errorf("database name is required")
	}
	if c.User == "" {
		return fmt.Errorf("database user is required")
	}
	return nil
}

// GetDefaultConfig returns a config with sensible defaults
func GetDefaultConfig() *Config {
	return &Config{
		Sync: SyncConfig{
			Strategy:  "full",
			BatchSize: 1000,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "console",
			Output: "stdout",
		},
	}
}
