package database

import (
	"context"
	"database/sql"
	"fmt"
)

// TruncateTable truncates a table
func (d *MySQLDriver) TruncateTable(ctx context.Context, tableName string) error {
	if d.db == nil {
		return ErrNotConnected
	}

	// MySQL requires disabling foreign key checks for TRUNCATE if referenced
	_, err := d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0")
	if err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}

	query := fmt.Sprintf("TRUNCATE TABLE %s", tableName)
	_, err = d.db.ExecContext(ctx, query)

	// Re-enable checks even if truncate failed
	_, _ = d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")

	return err
}

// GetMaxColumnValue returns the maximum value of a column
func (d *MySQLDriver) GetMaxColumnValue(ctx context.Context, tableName string, columnName string) (interface{}, error) {
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

	// MySQL driver often returns []byte for strings/text
	if b, ok := maxVal.([]byte); ok {
		return string(b), nil
	}

	return maxVal, nil
}
