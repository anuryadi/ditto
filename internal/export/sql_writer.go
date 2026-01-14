package export

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// SQLWriter writes data as SQL INSERT statements
type SQLWriter struct {
	output    io.Writer
	tableName string
	columns   []string
}

// NewSQLWriter creates a new SQL writer
func NewSQLWriter(output io.Writer, tableName string, columns []string) *SQLWriter {
	return &SQLWriter{
		output:    output,
		tableName: tableName,
		columns:   columns,
	}
}

func (w *SQLWriter) WriteHeader(columns []string) error {
	w.columns = columns
	// Write header comment
	_, err := fmt.Fprintf(w.output, "-- Export of table: %s\n-- Generated at: %s\n\n",
		w.tableName, time.Now().Format(time.RFC3339))
	return err
}

func (w *SQLWriter) WriteRow(values []interface{}) error {
	// Build INSERT statement
	valueStrings := make([]string, len(values))
	for i, v := range values {
		valueStrings[i] = formatSQLValue(v)
	}

	_, err := fmt.Fprintf(w.output, "INSERT INTO %s (%s) VALUES (%s);\n",
		w.tableName,
		strings.Join(w.columns, ", "),
		strings.Join(valueStrings, ", "),
	)
	return err
}

func (w *SQLWriter) Flush() error {
	return nil
}

func formatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case string:
		// Escape single quotes
		escaped := strings.ReplaceAll(val, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case []byte:
		escaped := strings.ReplaceAll(string(val), "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case time.Time:
		return fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05"))
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("'%v'", val)
	}
}
