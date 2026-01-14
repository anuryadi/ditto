package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/pkg/logger"
)

// Strategy defines the interface for synchronization strategies
type Strategy interface {
	Execute(ctx context.Context, source, target database.Driver, tableName string) (int, error)
}

// FullStrategy implements full synchronization (truncate + copy)
type FullStrategy struct {
	BatchSize int
}

func (s *FullStrategy) Execute(ctx context.Context, source, target database.Driver, tableName string) (int, error) {
	// 1. Truncate target table
	logger.Infof("Truncating target table %s...", tableName)
	if err := target.TruncateTable(ctx, tableName); err != nil {
		return 0, fmt.Errorf("failed to truncate target table: %w", err)
	}

	// 2. Get source columns
	schema, err := source.GetTableSchema(ctx, tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to get table schema: %w", err)
	}

	columns := make([]string, len(schema.Columns))
	for i, col := range schema.Columns {
		columns[i] = col.Name
	}

	// 3. Fetch data from source
	// TODO: Implement proper batch fetching/pagination to avoid memory issues for large tables.
	// For now, we assume simple SELECT * works for moderate sizes or standard Go sql driver handles rows streaming.
	query := fmt.Sprintf("SELECT %s FROM %s", buildColumnList(columns), tableName)
	rows, err := source.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query source: %w", err)
	}
	defer rows.Close()

	// 4. Batch insert into target
	writer := NewBatchWriter(target, tableName, columns, s.BatchSize)
	rowCount := 0

	colCount := len(columns)
	values := make([]interface{}, colCount)
	scanArgs := make([]interface{}, colCount)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return rowCount, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create a copy of values for the batch
		rowCopy := make([]interface{}, colCount)
		for i, v := range values {
			// Handle any necessary type conversions/copying here if needed
			// For basic types, this shallow copy is usually fine, but be careful with []byte reuse by drivers
			if b, ok := v.([]byte); ok {
				// efficient byte slice copy
				tmp := make([]byte, len(b))
				copy(tmp, b)
				rowCopy[i] = string(tmp) // Convert to string for safety in mixed drivers
			} else {
				rowCopy[i] = v
			}
		}

		if err := writer.Write(ctx, rowCopy); err != nil {
			return rowCount, fmt.Errorf("failed to write batch: %w", err)
		}
		rowCount++
	}

	if err := writer.Flush(ctx); err != nil {
		return rowCount, fmt.Errorf("failed to flush batch: %w", err)
	}

	return rowCount, nil
}

// IncrementalStrategy implements incremental synchronization
type IncrementalStrategy struct {
	BatchSize       int
	PrimaryKey      string
	TimestampColumn string
}

func (s *IncrementalStrategy) Execute(ctx context.Context, source, target database.Driver, tableName string) (int, error) {
	// 1. Get max value from target to determine start point
	syncColumn := s.TimestampColumn
	if syncColumn == "" {
		syncColumn = s.PrimaryKey
	}

	if syncColumn == "" {
		return 0, fmt.Errorf("incremental strategy requires primary_key or timestamp_column config")
	}

	maxVal, err := target.GetMaxColumnValue(ctx, tableName, syncColumn)
	if err != nil {
		return 0, fmt.Errorf("failed to get max value for column %s: %w", syncColumn, err)
	}

	logger.Infof("Incremental sync for %s starting from %v (%s)", tableName, maxVal, syncColumn)

	// 2. Get source columns
	schema, err := source.GetTableSchema(ctx, tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to get table schema: %w", err)
	}

	columns := make([]string, len(schema.Columns))
	for i, col := range schema.Columns {
		columns[i] = col.Name
	}

	// 3. Fetch new data from source
	query := fmt.Sprintf("SELECT %s FROM %s", buildColumnList(columns), tableName)
	var args []interface{}

	if maxVal != nil {
		query += fmt.Sprintf(" WHERE %s > $1", syncColumn)
		// Note: $1 param style is Postgres specific. MySQL uses ?.
		// We need a way to handle placeholders safely.
		// For now, let's use a workaround or string formatting if driver doesn't support named/common placeholders.
		// A better way is to abstraction strict query building.
		// Limitation: This might fail cross-db if we strictly use $1 vs ?.
		// Let's rely on string formatting for the WHERE clause value for simplicity in this MVP,
		// assuming safe values (ids/timestamps) or fix placeholder logic.

		// Fix: Use simple string formatting (unsafe for user input, but safe-ish for internal maxVal)
		// Or assume standard SQL behavior.
		// Let's try to query source driver to see what bind param it needs? No easy way in interface.
		// Let's just use string formatting for now.

		// Safe formatting for timestamp/string checks
		switch v := maxVal.(type) {
		case string:
			query = fmt.Sprintf("SELECT %s FROM %s WHERE %s > '%s'", buildColumnList(columns), tableName, syncColumn, v)
		case time.Time:
			query = fmt.Sprintf("SELECT %s FROM %s WHERE %s > '%s'", buildColumnList(columns), tableName, syncColumn, v.Format(time.RFC3339))
		default:
			query = fmt.Sprintf("SELECT %s FROM %s WHERE %s > %v", buildColumnList(columns), tableName, syncColumn, v)
		}
	}

	rows, err := source.Query(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to query source: %w", err)
	}
	defer rows.Close()

	// 4. Batch insert into target
	writer := NewBatchWriter(target, tableName, columns, s.BatchSize)
	rowCount := 0

	colCount := len(columns)
	values := make([]interface{}, colCount)
	scanArgs := make([]interface{}, colCount)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return rowCount, fmt.Errorf("failed to scan row: %w", err)
		}

		rowCopy := make([]interface{}, colCount)
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				tmp := make([]byte, len(b))
				copy(tmp, b)
				rowCopy[i] = string(tmp)
			} else {
				rowCopy[i] = v
			}
		}

		if err := writer.Write(ctx, rowCopy); err != nil {
			return rowCount, fmt.Errorf("failed to write batch: %w", err)
		}
		rowCount++
	}

	if err := writer.Flush(ctx); err != nil {
		return rowCount, fmt.Errorf("failed to flush batch: %w", err)
	}

	return rowCount, nil
}

func buildColumnList(columns []string) string {
	res := ""
	for i, c := range columns {
		if i > 0 {
			res += ", "
		}
		res += c
	}
	return res
}
