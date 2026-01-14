package export

import (
	"encoding/json"
	"io"
)

// JSONWriter writes data in JSON format
type JSONWriter struct {
	output    io.Writer
	tableName string
	columns   []string
	rows      []map[string]interface{}
}

// NewJSONWriter creates a new JSON writer
func NewJSONWriter(output io.Writer, tableName string) *JSONWriter {
	return &JSONWriter{
		output:    output,
		tableName: tableName,
		rows:      make([]map[string]interface{}, 0),
	}
}

func (w *JSONWriter) WriteHeader(columns []string) error {
	w.columns = columns
	return nil
}

func (w *JSONWriter) WriteRow(values []interface{}) error {
	row := make(map[string]interface{})
	for i, col := range w.columns {
		if i < len(values) {
			row[col] = values[i]
		}
	}
	w.rows = append(w.rows, row)
	return nil
}

func (w *JSONWriter) Flush() error {
	encoder := json.NewEncoder(w.output)
	encoder.SetIndent("", "  ")

	result := map[string]interface{}{
		"table": w.tableName,
		"count": len(w.rows),
		"data":  w.rows,
	}

	return encoder.Encode(result)
}
