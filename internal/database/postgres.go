package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver

	"github.com/anuryadi/ditto/internal/config"
	"github.com/anuryadi/ditto/pkg/logger"
)

// PostgresDriver implements Driver interface for PostgreSQL
type PostgresDriver struct {
	config *config.DatabaseConfig
	db     *sql.DB
}

// NewPostgresDriver creates a new PostgreSQL driver
func NewPostgresDriver(cfg *config.DatabaseConfig) (*PostgresDriver, error) {
	return &PostgresDriver{
		config: cfg,
	}, nil
}

// Connect establishes a connection to PostgreSQL
func (d *PostgresDriver) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.config.User,
		d.config.Password,
		d.config.Host,
		d.config.Port,
		d.config.Database,
		d.getSSLMode(),
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	d.db = db
	logger.Infof("Connected to PostgreSQL: %s@%s:%d/%s", d.config.User, d.config.Host, d.config.Port, d.config.Database)
	return nil
}

func (d *PostgresDriver) getSSLMode() string {
	if d.config.SSLMode == "" {
		return "disable"
	}
	return d.config.SSLMode
}

// Close closes the database connection
func (d *PostgresDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping tests the connection
func (d *PostgresDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return ErrNotConnected
	}
	return d.db.PingContext(ctx)
}

// GetTables returns all tables in the database
func (d *PostgresDriver) GetTables(ctx context.Context) ([]Table, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}

	query := `
		SELECT 
			table_name,
			table_schema,
			table_type,
			COALESCE(obj_description((table_schema || '.' || table_name)::regclass), '') as comment
		FROM information_schema.tables 
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name
	`

	rows, err := d.db.QueryContext(ctx, query)
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
func (d *PostgresDriver) GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
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

func (d *PostgresDriver) getColumns(ctx context.Context, tableName string) ([]Column, error) {
	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.udt_name || 
				CASE 
					WHEN c.character_maximum_length IS NOT NULL THEN '(' || c.character_maximum_length || ')'
					WHEN c.numeric_precision IS NOT NULL THEN '(' || c.numeric_precision || 
						CASE WHEN c.numeric_scale > 0 THEN ',' || c.numeric_scale ELSE '' END || ')'
					ELSE ''
				END as full_data_type,
			c.is_nullable = 'YES' as is_nullable,
			c.column_default,
			COALESCE(
				(SELECT true FROM information_schema.table_constraints tc
				 JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
				 WHERE tc.table_name = c.table_name AND tc.constraint_type = 'PRIMARY KEY' AND kcu.column_name = c.column_name
				 LIMIT 1), false
			) as is_primary_key,
			c.column_default LIKE 'nextval%' as is_auto_increment,
			COALESCE(pgd.description, '') as comment,
			c.ordinal_position
		FROM information_schema.columns c
		LEFT JOIN pg_catalog.pg_statio_all_tables st ON c.table_name = st.relname
		LEFT JOIN pg_catalog.pg_description pgd ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
		WHERE c.table_name = $1 AND c.table_schema = 'public'
		ORDER BY c.ordinal_position
	`

	rows, err := d.db.QueryContext(ctx, query, tableName)
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
func (d *PostgresDriver) GetPrimaryKey(ctx context.Context, tableName string) ([]string, error) {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name 
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY' 
			AND tc.table_name = $1
			AND tc.table_schema = 'public'
		ORDER BY kcu.ordinal_position
	`

	rows, err := d.db.QueryContext(ctx, query, tableName)
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

func (d *PostgresDriver) getIndexes(ctx context.Context, tableName string) ([]Index, error) {
	query := `
		SELECT
			i.relname as index_name,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as columns,
			ix.indisunique as is_unique,
			ix.indisprimary as is_primary
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1 AND t.relkind = 'r'
		GROUP BY i.relname, ix.indisunique, ix.indisprimary
	`

	rows, err := d.db.QueryContext(ctx, query, tableName)
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
		// Parse PostgreSQL array format {col1,col2}
		columnsStr = strings.Trim(columnsStr, "{}")
		if columnsStr != "" {
			idx.Columns = strings.Split(columnsStr, ",")
		}
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

func (d *PostgresDriver) getForeignKeys(ctx context.Context, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS referenced_table,
			ccu.column_name AS referenced_column,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu 
			ON ccu.constraint_name = tc.constraint_name
		JOIN information_schema.referential_constraints rc 
			ON rc.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' 
			AND tc.table_name = $1
			AND tc.table_schema = 'public'
	`

	rows, err := d.db.QueryContext(ctx, query, tableName)
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
func (d *PostgresDriver) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}
	return d.db.QueryContext(ctx, query, args...)
}

// Execute executes a query that doesn't return rows
func (d *PostgresDriver) Execute(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}
	return d.db.ExecContext(ctx, query, args...)
}

// BulkInsert performs bulk insert operation
func (d *PostgresDriver) BulkInsert(ctx context.Context, tableName string, columns []string, rows [][]interface{}) error {
	if d.db == nil {
		return ErrNotConnected
	}

	if len(rows) == 0 {
		return nil
	}

	// Build INSERT statement with multiple value sets
	var placeholders []string
	var values []interface{}
	paramCount := 1

	for _, row := range rows {
		var rowPlaceholders []string
		for range row {
			rowPlaceholders = append(rowPlaceholders, fmt.Sprintf("$%d", paramCount))
			paramCount++
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
func (d *PostgresDriver) BeginTx(ctx context.Context) (*sql.Tx, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}
	return d.db.BeginTx(ctx, nil)
}

// DriverName returns the driver name
func (d *PostgresDriver) DriverName() string {
	return "postgres"
}

// DatabaseName returns the database name
func (d *PostgresDriver) DatabaseName() string {
	return d.config.Database
}
