package schema

import "github.com/anuryadi/ditto/internal/database"

// Difference represents a schema difference between source and target
type Difference struct {
	Type        DifferenceType `json:"type"`
	ObjectType  string         `json:"object_type"` // table, column, index, foreign_key
	ObjectName  string         `json:"object_name"`
	TableName   string         `json:"table_name,omitempty"`
	Source      interface{}    `json:"source,omitempty"`
	Target      interface{}    `json:"target,omitempty"`
	Description string         `json:"description"`
}

// DifferenceType represents the type of difference
type DifferenceType string

const (
	DiffAdded    DifferenceType = "added"    // Exists in source, not in target
	DiffRemoved  DifferenceType = "removed"  // Exists in target, not in source
	DiffModified DifferenceType = "modified" // Exists in both but different
)

// CompareResult represents the result of a schema comparison
type CompareResult struct {
	SourceDB    string       `json:"source_db"`
	TargetDB    string       `json:"target_db"`
	Differences []Difference `json:"differences"`
	Summary     Summary      `json:"summary"`
}

// Summary provides a summary of differences
type Summary struct {
	TablesAdded    int `json:"tables_added"`
	TablesRemoved  int `json:"tables_removed"`
	ColumnsAdded   int `json:"columns_added"`
	ColumnsRemoved int `json:"columns_removed"`
	ColumnsChanged int `json:"columns_changed"`
	IndexesAdded   int `json:"indexes_added"`
	IndexesRemoved int `json:"indexes_removed"`
	TotalDiffs     int `json:"total_diffs"`
}

// TableComparison holds comparison data for a single table
type TableComparison struct {
	Name         string
	SourceSchema *database.TableSchema
	TargetSchema *database.TableSchema
}
