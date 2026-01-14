package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anuryadi/ditto/internal/config"
)

// Driver defines the interface for database operations
type Driver interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error

	// Schema operations
	GetTables(ctx context.Context) ([]Table, error)
	GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error)
	GetPrimaryKey(ctx context.Context, tableName string) ([]string, error)

	// Data operations
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	Execute(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Batch operations
	BulkInsert(ctx context.Context, tableName string, columns []string, rows [][]interface{}) error

	// Transaction support
	BeginTx(ctx context.Context) (*sql.Tx, error)

	// Metadata
	DriverName() string
	DatabaseName() string
}

// Table represents a database table
type Table struct {
	Name    string
	Schema  string
	Type    string // TABLE, VIEW, etc.
	Comment string
}

// TableSchema represents the schema of a table
type TableSchema struct {
	Name        string
	Columns     []Column
	PrimaryKey  []string
	Indexes     []Index
	ForeignKeys []ForeignKey
}

// Column represents a table column
type Column struct {
	Name            string
	DataType        string
	FullDataType    string // includes precision, scale, etc.
	IsNullable      bool
	DefaultValue    *string
	IsPrimaryKey    bool
	IsAutoIncrement bool
	Comment         string
	OrdinalPosition int
}

// Index represents a table index
type Index struct {
	Name      string
	Columns   []string
	IsUnique  bool
	IsPrimary bool
}

// ForeignKey represents a foreign key constraint
type ForeignKey struct {
	Name              string
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
	OnUpdate          string
	OnDelete          string
}

// DriverFactory creates a new database driver based on configuration
func NewDriver(cfg *config.DatabaseConfig) (Driver, error) {
	switch cfg.Driver {
	case "postgres", "postgresql":
		return NewPostgresDriver(cfg)
	case "mysql", "mariadb":
		return NewMySQLDriver(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

// Common error types
var (
	ErrNotConnected  = fmt.Errorf("database not connected")
	ErrTableNotFound = fmt.Errorf("table not found")
)
