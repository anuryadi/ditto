package export

import (
	"encoding/csv"
	"fmt"
	"io"
)

// CSVWriter writes data in CSV format
type CSVWriter struct {
	writer *csv.Writer
}

// NewCSVWriter creates a new CSV writer
func NewCSVWriter(output io.Writer) *CSVWriter {
	return &CSVWriter{
		writer: csv.NewWriter(output),
	}
}

func (w *CSVWriter) WriteHeader(columns []string) error {
	return w.writer.Write(columns)
}

func (w *CSVWriter) WriteRow(values []interface{}) error {
	record := make([]string, len(values))
	for i, v := range values {
		record[i] = fmt.Sprintf("%v", v)
	}
	return w.writer.Write(record)
}

func (w *CSVWriter) Flush() error {
	w.writer.Flush()
	return w.writer.Error()
}
