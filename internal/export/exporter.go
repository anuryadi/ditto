package export

import (
	"context"
	"fmt"
	"io"

	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/pkg/logger"
)

// Format represents the export format type
type Format string

const (
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
	FormatSQL  Format = "sql"
)

// Writer defines the interface for export format writers
type Writer interface {
	WriteHeader(columns []string) error
	WriteRow(values []interface{}) error
	Flush() error
}

// Options holds export configuration
type Options struct {
	Format    Format
	TableName string
	Query     string // Custom query (optional)
	Compress  bool
}

// Exporter handles data export from database
type Exporter struct {
	driver  database.Driver
	options Options
	output  io.Writer
}

// NewExporter creates a new exporter
func NewExporter(driver database.Driver, options Options, output io.Writer) *Exporter {
	return &Exporter{
		driver:  driver,
		options: options,
		output:  output,
	}
}

// Export performs the data export
func (e *Exporter) Export(ctx context.Context) (int, error) {
	// 1. Get table schema
	schema, err := e.driver.GetTableSchema(ctx, e.options.TableName)
	if err != nil {
		return 0, fmt.Errorf("failed to get table schema: %w", err)
	}

	columns := make([]string, len(schema.Columns))
	for i, col := range schema.Columns {
		columns[i] = col.Name
	}

	// 2. Build query
	query := e.options.Query
	if query == "" {
		query = fmt.Sprintf("SELECT * FROM %s", e.options.TableName)
	}

	// 3. Execute query
	rows, err := e.driver.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query data: %w", err)
	}
	defer rows.Close()

	// 4. Create format writer
	writer, err := e.createWriter(columns)
	if err != nil {
		return 0, err
	}

	// 5. Write header
	if err := writer.WriteHeader(columns); err != nil {
		return 0, fmt.Errorf("failed to write header: %w", err)
	}

	// 6. Write rows
	rowCount := 0
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return rowCount, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert []byte to string for readability
		exportValues := make([]interface{}, len(values))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				exportValues[i] = string(b)
			} else {
				exportValues[i] = v
			}
		}

		if err := writer.WriteRow(exportValues); err != nil {
			return rowCount, fmt.Errorf("failed to write row: %w", err)
		}
		rowCount++
	}

	// 7. Flush
	if err := writer.Flush(); err != nil {
		return rowCount, fmt.Errorf("failed to flush: %w", err)
	}

	logger.Infof("Exported %d rows from table %s", rowCount, e.options.TableName)
	return rowCount, nil
}

func (e *Exporter) createWriter(columns []string) (Writer, error) {
	switch e.options.Format {
	case FormatJSON:
		return NewJSONWriter(e.output, e.options.TableName), nil
	case FormatCSV:
		return NewCSVWriter(e.output), nil
	case FormatSQL:
		return NewSQLWriter(e.output, e.options.TableName, columns), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", e.options.Format)
	}
}
