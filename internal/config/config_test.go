package config

import (
	"testing"
)

func TestParseDSN_Postgres(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected *DatabaseConfig
		wantErr  bool
	}{
		{
			name: "full postgres dsn",
			dsn:  "postgres://user:password@localhost:5432/mydb?sslmode=disable",
			expected: &DatabaseConfig{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "password",
				SSLMode:  "disable",
			},
			wantErr: false,
		},
		{
			name: "postgres with default port",
			dsn:  "postgres://user:password@localhost/mydb",
			expected: &DatabaseConfig{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "password",
			},
			wantErr: false,
		},
		{
			name: "postgresql scheme",
			dsn:  "postgresql://user:pass@host:5433/db",
			expected: &DatabaseConfig{
				Driver:   "postgres",
				Host:     "host",
				Port:     5433,
				Database: "db",
				User:     "user",
				Password: "pass",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDSN(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Driver != tt.expected.Driver {
				t.Errorf("Driver = %v, want %v", got.Driver, tt.expected.Driver)
			}
			if got.Host != tt.expected.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.expected.Host)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.expected.Port)
			}
			if got.Database != tt.expected.Database {
				t.Errorf("Database = %v, want %v", got.Database, tt.expected.Database)
			}
			if got.User != tt.expected.User {
				t.Errorf("User = %v, want %v", got.User, tt.expected.User)
			}
		})
	}
}

func TestParseDSN_MySQL(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected *DatabaseConfig
	}{
		{
			name: "mysql dsn",
			dsn:  "mysql://root:password@localhost:3306/testdb",
			expected: &DatabaseConfig{
				Driver:   "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
				User:     "root",
				Password: "password",
			},
		},
		{
			name: "mysql with default port",
			dsn:  "mysql://root:password@localhost/testdb",
			expected: &DatabaseConfig{
				Driver:   "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
				User:     "root",
				Password: "password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("ParseDSN() error = %v", err)
			}

			if got.Driver != tt.expected.Driver {
				t.Errorf("Driver = %v, want %v", got.Driver, tt.expected.Driver)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.expected.Port)
			}
		})
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	tests := []struct {
		name     string
		config   *DatabaseConfig
		expected string
	}{
		{
			name: "postgres dsn",
			config: &DatabaseConfig{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				SSLMode:  "disable",
			},
			expected: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
		},
		{
			name: "mysql dsn",
			config: &DatabaseConfig{
				Driver:   "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "mydb",
				User:     "root",
				Password: "pass",
			},
			expected: "root:pass@tcp(localhost:3306)/mydb?parseTime=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DSN()
			if got != tt.expected {
				t.Errorf("DSN() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDatabaseConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *DatabaseConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &DatabaseConfig{
				Driver:   "postgres",
				Host:     "localhost",
				Database: "mydb",
				User:     "user",
			},
			wantErr: false,
		},
		{
			name: "missing driver",
			config: &DatabaseConfig{
				Host:     "localhost",
				Database: "mydb",
				User:     "user",
			},
			wantErr: true,
		},
		{
			name: "missing host",
			config: &DatabaseConfig{
				Driver:   "postgres",
				Database: "mydb",
				User:     "user",
			},
			wantErr: true,
		},
		{
			name: "missing database",
			config: &DatabaseConfig{
				Driver: "postgres",
				Host:   "localhost",
				User:   "user",
			},
			wantErr: true,
		},
		{
			name: "missing user",
			config: &DatabaseConfig{
				Driver:   "postgres",
				Host:     "localhost",
				Database: "mydb",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.Sync.Strategy != "full" {
		t.Errorf("default strategy = %v, want full", cfg.Sync.Strategy)
	}
	if cfg.Sync.BatchSize != 1000 {
		t.Errorf("default batch size = %v, want 1000", cfg.Sync.BatchSize)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("default log level = %v, want info", cfg.Logging.Level)
	}
}
