package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MySQL driver

	"github.com/anuryadi/ditto/internal/config"
	"github.com/anuryadi/ditto/pkg/logger"
)

// MySQLDriver implements Driver interface for MySQL/MariaDB
type MySQLDriver struct {
	config *config.DatabaseConfig
	db     *sql.DB
}

// NewMySQLDriver creates a new MySQL driver
func NewMySQLDriver(cfg *config.DatabaseConfig) (*MySQLDriver, error) {
	return &MySQLDriver{
		config: cfg,
	}, nil
}

// Connect establishes a connection to MySQL
func (d *MySQLDriver) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		d.config.User,
		d.config.Password,
		d.config.Host,
		d.config.Port,
		d.config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open mysql connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping mysql: %w", err)
	}

	d.db = db
	logger.Infof("Connected to MySQL: %s@%s:%d/%s", d.config.User, d.config.Host, d.config.Port, d.config.Database)
	return nil
}

// Close closes the database connection
func (d *MySQLDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping tests the connection
func (d *MySQLDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return ErrNotConnected
	}
	return d.db.PingContext(ctx)
}

// GetTables returns all tables in the database
func (d *MySQLDriver) GetTables(ctx context.Context) ([]Table, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}

	query := `
		SELECT 
			TABLE_NAME,
			TABLE_SCHEMA,
			TABLE_TYPE,
			COALESCE(TABLE_COMMENT, '') as comment
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`

	rows, err := d.db.QueryContext(ctx, query, d.config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name, &t.Schema, &t.Type, &t.Comment); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// GetTableSchema returns the schema of a specific table
func (d *MySQLDriver) GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}

	schema := &TableSchema{Name: tableName}

	// Get columns
	columns, err := d.getColumns(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.Columns = columns

	// Get primary key
	pk, err := d.GetPrimaryKey(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.PrimaryKey = pk

	// Get indexes
	indexes, err := d.getIndexes(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.Indexes = indexes

	// Get foreign keys
	fks, err := d.getForeignKeys(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.ForeignKeys = fks

	return schema, nil
}

func (d *MySQLDriver) getColumns(ctx context.Context, tableName string) ([]Column, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			COLUMN_TYPE,
			IS_NULLABLE = 'YES' as is_nullable,
			COLUMN_DEFAULT,
			COLUMN_KEY = 'PRI' as is_primary_key,
			EXTRA LIKE '%auto_increment%' as is_auto_increment,
			COALESCE(COLUMN_COMMENT, '') as comment,
			ORDINAL_POSITION
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := d.db.QueryContext(ctx, query, d.config.Database, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var defaultValue sql.NullString
		if err := rows.Scan(
			&col.Name,
			&col.DataType,
			&col.FullDataType,
			&col.IsNullable,
			&defaultValue,
			&col.IsPrimaryKey,
			&col.IsAutoIncrement,
			&col.Comment,
			&col.OrdinalPosition,
		); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// GetPrimaryKey returns the primary key columns of a table
func (d *MySQLDriver) GetPrimaryKey(ctx context.Context, tableName string) ([]string, error) {
	query := `
		SELECT COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? 
			AND TABLE_NAME = ?
			AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION
	`

	rows, err := d.db.QueryContext(ctx, query, d.config.Database, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary key: %w", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, fmt.Errorf("failed to scan primary key column: %w", err)
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (d *MySQLDriver) getIndexes(ctx context.Context, tableName string) ([]Index, error) {
	query := `
		SELECT
			INDEX_NAME,
			GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) as columns,
			NOT NON_UNIQUE as is_unique,
			INDEX_NAME = 'PRIMARY' as is_primary
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		GROUP BY INDEX_NAME, NON_UNIQUE
	`

	rows, err := d.db.QueryContext(ctx, query, d.config.Database, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
		var columnsStr string
		if err := rows.Scan(&idx.Name, &columnsStr, &idx.IsUnique, &idx.IsPrimary); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}
		if columnsStr != "" {
			idx.Columns = strings.Split(columnsStr, ",")
		}
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

func (d *MySQLDriver) getForeignKeys(ctx context.Context, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT
			kcu.CONSTRAINT_NAME,
			kcu.COLUMN_NAME,
			kcu.REFERENCED_TABLE_NAME,
			kcu.REFERENCED_COLUMN_NAME,
			rc.UPDATE_RULE,
			rc.DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc 
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME 
			AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ?
			AND kcu.TABLE_NAME = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION
	`

	rows, err := d.db.QueryContext(ctx, query, d.config.Database, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	fkMap := make(map[string]*ForeignKey)
	for rows.Next() {
		var name, column, refTable, refColumn, onUpdate, onDelete string
		if err := rows.Scan(&name, &column, &refTable, &refColumn, &onUpdate, &onDelete); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}

		if fk, exists := fkMap[name]; exists {
			fk.Columns = append(fk.Columns, column)
			fk.ReferencedColumns = append(fk.ReferencedColumns, refColumn)
		} else {
			fkMap[name] = &ForeignKey{
				Name:              name,
				Columns:           []string{column},
				ReferencedTable:   refTable,
				ReferencedColumns: []string{refColumn},
				OnUpdate:          onUpdate,
				OnDelete:          onDelete,
			}
		}
	}

	var foreignKeys []ForeignKey
	for _, fk := range fkMap {
		foreignKeys = append(foreignKeys, *fk)
	}

	return foreignKeys, rows.Err()
}

// Query executes a query and returns rows
func (d *MySQLDriver) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}
	return d.db.QueryContext(ctx, query, args...)
}

// Execute executes a query that doesn't return rows
func (d *MySQLDriver) Execute(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}
	return d.db.ExecContext(ctx, query, args...)
}

// BulkInsert performs bulk insert operation
func (d *MySQLDriver) BulkInsert(ctx context.Context, tableName string, columns []string, rows [][]interface{}) error {
	if d.db == nil {
		return ErrNotConnected
	}

	if len(rows) == 0 {
		return nil
	}

	// Build INSERT statement with multiple value sets
	var placeholders []string
	var values []interface{}

	for _, row := range rows {
		rowPlaceholders := make([]string, len(row))
		for i := range row {
			rowPlaceholders[i] = "?"
		}
		placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")
		values = append(values, row...)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := d.db.ExecContext(ctx, query, values...)
	return err
}

// BeginTx starts a new transaction
func (d *MySQLDriver) BeginTx(ctx context.Context) (*sql.Tx, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}
	return d.db.BeginTx(ctx, nil)
}

// DriverName returns the driver name
func (d *MySQLDriver) DriverName() string {
	return "mysql"
}

// DatabaseName returns the database name
func (d *MySQLDriver) DatabaseName() string {
	return d.config.Database
}
