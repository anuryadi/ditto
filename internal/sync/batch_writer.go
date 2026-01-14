package sync

import (
	"context"
	"fmt"
)

// BulkInserter defines the interface for bulk insert operations
type BulkInserter interface {
	BulkInsert(ctx context.Context, tableName string, columns []string, rows [][]interface{}) error
}

// BatchWriter buffers rows and writes them in batches
type BatchWriter struct {
	inserter  BulkInserter
	tableName string
	columns   []string
	batchSize int
	buffer    [][]interface{}
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(inserter BulkInserter, tableName string, columns []string, batchSize int) *BatchWriter {
	return &BatchWriter{
		inserter:  inserter,
		tableName: tableName,
		columns:   columns,
		batchSize: batchSize,
		buffer:    make([][]interface{}, 0, batchSize),
	}
}

// Write adds a row to the buffer and flushes if full
func (w *BatchWriter) Write(ctx context.Context, row []interface{}) error {
	w.buffer = append(w.buffer, row)

	if len(w.buffer) >= w.batchSize {
		return w.Flush(ctx)
	}
	return nil
}

// Flush writes any buffered rows to the database
func (w *BatchWriter) Flush(ctx context.Context) error {
	if len(w.buffer) == 0 {
		return nil
	}

	if err := w.inserter.BulkInsert(ctx, w.tableName, w.columns, w.buffer); err != nil {
		return fmt.Errorf("bulk insert failed: %w", err)
	}

	// Reset buffer
	w.buffer = make([][]interface{}, 0, w.batchSize)
	return nil
}
