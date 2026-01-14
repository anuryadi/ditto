package schema

import (
	"testing"

	"github.com/anuryadi/ditto/internal/database"
)

func TestNormalizeDataType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"int4", "integer"},
		{"int8", "bigint"},
		{"INT4", "integer"},
		{"varchar", "varchar"},
		{"character varying", "varchar"},
		{"bool", "boolean"},
		{"BOOL", "boolean"},
		{"timestamp", "timestamp"},
		{"timestamptz", "timestamp"},
		{"datetime", "timestamp"},
		{"unknown_type", "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDataType(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeDataType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestComparator_compareColumn(t *testing.T) {
	c := &Comparator{}

	defaultVal := "0"

	tests := []struct {
		name        string
		source      database.Column
		target      database.Column
		wantChanges int
	}{
		{
			name: "identical columns",
			source: database.Column{
				Name:       "id",
				DataType:   "integer",
				IsNullable: false,
			},
			target: database.Column{
				Name:       "id",
				DataType:   "integer",
				IsNullable: false,
			},
			wantChanges: 0,
		},
		{
			name: "different type",
			source: database.Column{
				Name:     "id",
				DataType: "integer",
			},
			target: database.Column{
				Name:     "id",
				DataType: "bigint",
			},
			wantChanges: 1,
		},
		{
			name: "different nullable",
			source: database.Column{
				Name:       "name",
				DataType:   "varchar",
				IsNullable: true,
			},
			target: database.Column{
				Name:       "name",
				DataType:   "varchar",
				IsNullable: false,
			},
			wantChanges: 1,
		},
		{
			name: "different default",
			source: database.Column{
				Name:         "status",
				DataType:     "integer",
				DefaultValue: &defaultVal,
			},
			target: database.Column{
				Name:         "status",
				DataType:     "integer",
				DefaultValue: nil,
			},
			wantChanges: 1,
		},
		{
			name: "multiple differences",
			source: database.Column{
				Name:       "id",
				DataType:   "integer",
				IsNullable: false,
			},
			target: database.Column{
				Name:       "id",
				DataType:   "bigint",
				IsNullable: true,
			},
			wantChanges: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := c.compareColumn(tt.source, tt.target)
			if len(changes) != tt.wantChanges {
				t.Errorf("compareColumn() got %d changes, want %d: %v", len(changes), tt.wantChanges, changes)
			}
		})
	}
}

func TestComparator_compareColumns(t *testing.T) {
	c := &Comparator{}

	source := []database.Column{
		{Name: "id", DataType: "integer"},
		{Name: "name", DataType: "varchar"},
		{Name: "email", DataType: "varchar"}, // only in source
	}

	target := []database.Column{
		{Name: "id", DataType: "bigint"}, // modified type
		{Name: "name", DataType: "varchar"},
		{Name: "phone", DataType: "varchar"}, // only in target
	}

	diffs := c.compareColumns("users", source, target)

	// Should have: 1 added (email), 1 removed (phone), 1 modified (id type)
	if len(diffs) != 3 {
		t.Errorf("compareColumns() got %d diffs, want 3", len(diffs))
	}

	// Count by type
	added := 0
	removed := 0
	modified := 0
	for _, diff := range diffs {
		switch diff.Type {
		case DiffAdded:
			added++
		case DiffRemoved:
			removed++
		case DiffModified:
			modified++
		}
	}

	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if modified != 1 {
		t.Errorf("modified = %d, want 1", modified)
	}
}

func TestComparator_compareIndexes(t *testing.T) {
	c := &Comparator{}

	source := []database.Index{
		{Name: "idx_users_email", Columns: []string{"email"}, IsUnique: true},
		{Name: "idx_users_name", Columns: []string{"name"}},
	}

	target := []database.Index{
		{Name: "idx_users_email", Columns: []string{"email"}, IsUnique: true},
		{Name: "idx_users_phone", Columns: []string{"phone"}},
	}

	diffs := c.compareIndexes("users", source, target)

	// Should have: 1 added (idx_users_name), 1 removed (idx_users_phone)
	if len(diffs) != 2 {
		t.Errorf("compareIndexes() got %d diffs, want 2", len(diffs))
	}
}
