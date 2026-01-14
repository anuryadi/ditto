package database

import (
	"context"
	"database/sql"
	"fmt"
)

// TruncateTable truncates a table
func (d *PostgresDriver) TruncateTable(ctx context.Context, tableName string) error {
	if d.db == nil {
		return ErrNotConnected
	}

	query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tableName)
	_, err := d.db.ExecContext(ctx, query)
	return err
}

// GetMaxColumnValue returns the maximum value of a column
func (d *PostgresDriver) GetMaxColumnValue(ctx context.Context, tableName string, columnName string) (interface{}, error) {
	if d.db == nil {
		return nil, ErrNotConnected
	}

	query := fmt.Sprintf("SELECT MAX(%s) FROM %s", columnName, tableName)
	var maxVal interface{}
	err := d.db.QueryRowContext(ctx, query).Scan(&maxVal)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return maxVal, nil
}
