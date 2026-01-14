package sync

import (
	"context"
	"testing"
)

// MockBulkInserter is a simple mock for testing
type MockBulkInserter struct {
	BulkInsertFunc func(ctx context.Context, tableName string, columns []string, rows [][]interface{}) error
	Calls          int
}

func (m *MockBulkInserter) BulkInsert(ctx context.Context, tableName string, columns []string, rows [][]interface{}) error {
	m.Calls++
	if m.BulkInsertFunc != nil {
		return m.BulkInsertFunc(ctx, tableName, columns, rows)
	}
	return nil
}

func TestBatchWriter_WriteAndFlush(t *testing.T) {
	batchSize := 2
	tableName := "users"
	columns := []string{"id", "name"}

	// Track inserted rows
	var insertedRows [][]interface{}

	mock := &MockBulkInserter{
		BulkInsertFunc: func(ctx context.Context, table string, cols []string, rows [][]interface{}) error {
			if table != tableName {
				t.Errorf("expected table %s, got %s", tableName, table)
			}
			// Copy rows to avoid reference issues if buffer is reused improperly (though BatchWriter reallocates)
			rowsCopy := make([][]interface{}, len(rows))
			copy(rowsCopy, rows)
			insertedRows = append(insertedRows, rowsCopy...)
			return nil
		},
	}

	writer := NewBatchWriter(mock, tableName, columns, batchSize)
	ctx := context.Background()

	// Write 1st row (buffer: 1)
	writer.Write(ctx, []interface{}{1, "Alice"})
	if len(insertedRows) != 0 {
		t.Errorf("expected 0 rows inserted, got %d", len(insertedRows))
	}

	// Write 2nd row (buffer: 2 -> Flush)
	writer.Write(ctx, []interface{}{2, "Bob"})
	if len(insertedRows) != 2 {
		t.Errorf("expected 2 rows inserted, got %d", len(insertedRows))
	}

	// Write 3rd row (buffer: 1)
	writer.Write(ctx, []interface{}{3, "Charlie"})
	if len(insertedRows) != 2 {
		t.Errorf("expected 2 rows inserted (pending flush), got %d", len(insertedRows))
	}

	// Manual Flush
	writer.Flush(ctx)
	if len(insertedRows) != 3 {
		t.Errorf("expected 3 rows inserted, got %d", len(insertedRows))
	}

	// Verify mock calls
	if mock.Calls != 2 {
		t.Errorf("expected 2 BulkInsert calls (1 auto, 1 manual), got %d", mock.Calls)
	}
}

func TestBatchWriter_EmptyFlush(t *testing.T) {
	mock := &MockBulkInserter{}
	writer := NewBatchWriter(mock, "t", []string{"c"}, 10)

	// Flush empty buffer should do nothing
	writer.Flush(context.Background())

	if mock.Calls != 0 {
		t.Errorf("expected 0 calls for empty flush, got %d", mock.Calls)
	}
}
