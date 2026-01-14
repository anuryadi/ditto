package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/pkg/logger"
)

// Comparator compares schemas between two databases
type Comparator struct {
	source database.Driver
	target database.Driver
}

// NewComparator creates a new schema comparator
func NewComparator(source, target database.Driver) *Comparator {
	return &Comparator{
		source: source,
		target: target,
	}
}

// Compare performs a full schema comparison between source and target
func (c *Comparator) Compare(ctx context.Context) (*CompareResult, error) {
	result := &CompareResult{
		SourceDB:    c.source.DatabaseName(),
		TargetDB:    c.target.DatabaseName(),
		Differences: []Difference{},
	}

	// Get tables from both databases
	logger.Info("Fetching tables from source database...")
	sourceTables, err := c.source.GetTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get source tables: %w", err)
	}

	logger.Info("Fetching tables from target database...")
	targetTables, err := c.target.GetTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get target tables: %w", err)
	}

	// Create maps for easier lookup
	sourceTableMap := make(map[string]database.Table)
	targetTableMap := make(map[string]database.Table)

	for _, t := range sourceTables {
		sourceTableMap[t.Name] = t
	}
	for _, t := range targetTables {
		targetTableMap[t.Name] = t
	}

	// Find tables added (in source, not in target)
	for name := range sourceTableMap {
		if _, exists := targetTableMap[name]; !exists {
			result.Differences = append(result.Differences, Difference{
				Type:        DiffAdded,
				ObjectType:  "table",
				ObjectName:  name,
				Description: fmt.Sprintf("Table '%s' exists in source but not in target", name),
			})
			result.Summary.TablesAdded++
		}
	}

	// Find tables removed (in target, not in source)
	for name := range targetTableMap {
		if _, exists := sourceTableMap[name]; !exists {
			result.Differences = append(result.Differences, Difference{
				Type:        DiffRemoved,
				ObjectType:  "table",
				ObjectName:  name,
				Description: fmt.Sprintf("Table '%s' exists in target but not in source", name),
			})
			result.Summary.TablesRemoved++
		}
	}

	// Compare tables that exist in both
	for name := range sourceTableMap {
		if _, exists := targetTableMap[name]; exists {
			logger.Debugf("Comparing table: %s", name)
			diffs, err := c.compareTable(ctx, name)
			if err != nil {
				return nil, fmt.Errorf("failed to compare table %s: %w", name, err)
			}
			result.Differences = append(result.Differences, diffs...)

			// Update summary
			for _, diff := range diffs {
				switch diff.ObjectType {
				case "column":
					switch diff.Type {
					case DiffAdded:
						result.Summary.ColumnsAdded++
					case DiffRemoved:
						result.Summary.ColumnsRemoved++
					case DiffModified:
						result.Summary.ColumnsChanged++
					}
				case "index":
					switch diff.Type {
					case DiffAdded:
						result.Summary.IndexesAdded++
					case DiffRemoved:
						result.Summary.IndexesRemoved++
					}
				}
			}
		}
	}

	result.Summary.TotalDiffs = len(result.Differences)
	return result, nil
}

// compareTable compares the schema of a single table
func (c *Comparator) compareTable(ctx context.Context, tableName string) ([]Difference, error) {
	var diffs []Difference

	// Get schemas
	sourceSchema, err := c.source.GetTableSchema(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get source schema: %w", err)
	}

	targetSchema, err := c.target.GetTableSchema(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get target schema: %w", err)
	}

	// Compare columns
	columnDiffs := c.compareColumns(tableName, sourceSchema.Columns, targetSchema.Columns)
	diffs = append(diffs, columnDiffs...)

	// Compare indexes
	indexDiffs := c.compareIndexes(tableName, sourceSchema.Indexes, targetSchema.Indexes)
	diffs = append(diffs, indexDiffs...)

	return diffs, nil
}

// compareColumns compares columns between source and target
func (c *Comparator) compareColumns(tableName string, source, target []database.Column) []Difference {
	var diffs []Difference

	sourceMap := make(map[string]database.Column)
	targetMap := make(map[string]database.Column)

	for _, col := range source {
		sourceMap[col.Name] = col
	}
	for _, col := range target {
		targetMap[col.Name] = col
	}

	// Find added columns
	for name, col := range sourceMap {
		if _, exists := targetMap[name]; !exists {
			diffs = append(diffs, Difference{
				Type:        DiffAdded,
				ObjectType:  "column",
				ObjectName:  name,
				TableName:   tableName,
				Source:      col,
				Description: fmt.Sprintf("Column '%s.%s' exists in source but not in target", tableName, name),
			})
		}
	}

	// Find removed columns
	for name, col := range targetMap {
		if _, exists := sourceMap[name]; !exists {
			diffs = append(diffs, Difference{
				Type:        DiffRemoved,
				ObjectType:  "column",
				ObjectName:  name,
				TableName:   tableName,
				Target:      col,
				Description: fmt.Sprintf("Column '%s.%s' exists in target but not in source", tableName, name),
			})
		}
	}

	// Find modified columns
	for name, sourceCol := range sourceMap {
		if targetCol, exists := targetMap[name]; exists {
			changes := c.compareColumn(sourceCol, targetCol)
			if len(changes) > 0 {
				diffs = append(diffs, Difference{
					Type:        DiffModified,
					ObjectType:  "column",
					ObjectName:  name,
					TableName:   tableName,
					Source:      sourceCol,
					Target:      targetCol,
					Description: fmt.Sprintf("Column '%s.%s' differs: %s", tableName, name, strings.Join(changes, ", ")),
				})
			}
		}
	}

	return diffs
}

// compareColumn compares two columns and returns list of differences
func (c *Comparator) compareColumn(source, target database.Column) []string {
	var changes []string

	// Normalize data types for comparison across different databases
	sourceType := normalizeDataType(source.DataType)
	targetType := normalizeDataType(target.DataType)

	if sourceType != targetType {
		changes = append(changes, fmt.Sprintf("type: %s -> %s", source.DataType, target.DataType))
	}

	if source.IsNullable != target.IsNullable {
		changes = append(changes, fmt.Sprintf("nullable: %v -> %v", source.IsNullable, target.IsNullable))
	}

	// Compare default values (with nil handling)
	sourceDefault := ""
	targetDefault := ""
	if source.DefaultValue != nil {
		sourceDefault = *source.DefaultValue
	}
	if target.DefaultValue != nil {
		targetDefault = *target.DefaultValue
	}
	if sourceDefault != targetDefault {
		changes = append(changes, fmt.Sprintf("default: '%s' -> '%s'", sourceDefault, targetDefault))
	}

	return changes
}

// compareIndexes compares indexes between source and target
func (c *Comparator) compareIndexes(tableName string, source, target []database.Index) []Difference {
	var diffs []Difference

	sourceMap := make(map[string]database.Index)
	targetMap := make(map[string]database.Index)

	for _, idx := range source {
		sourceMap[idx.Name] = idx
	}
	for _, idx := range target {
		targetMap[idx.Name] = idx
	}

	// Find added indexes
	for name, idx := range sourceMap {
		if _, exists := targetMap[name]; !exists {
			diffs = append(diffs, Difference{
				Type:        DiffAdded,
				ObjectType:  "index",
				ObjectName:  name,
				TableName:   tableName,
				Source:      idx,
				Description: fmt.Sprintf("Index '%s' on table '%s' exists in source but not in target", name, tableName),
			})
		}
	}

	// Find removed indexes
	for name, idx := range targetMap {
		if _, exists := sourceMap[name]; !exists {
			diffs = append(diffs, Difference{
				Type:        DiffRemoved,
				ObjectType:  "index",
				ObjectName:  name,
				TableName:   tableName,
				Target:      idx,
				Description: fmt.Sprintf("Index '%s' on table '%s' exists in target but not in source", name, tableName),
			})
		}
	}

	return diffs
}

// normalizeDataType normalizes data type names across databases
func normalizeDataType(dataType string) string {
	dataType = strings.ToLower(dataType)

	// Map common equivalent types
	typeMap := map[string]string{
		"int4":              "integer",
		"int8":              "bigint",
		"int2":              "smallint",
		"float8":            "double",
		"float4":            "float",
		"bool":              "boolean",
		"varchar":           "varchar",
		"character varying": "varchar",
		"text":              "text",
		"timestamp":         "timestamp",
		"timestamptz":       "timestamp",
		"datetime":          "timestamp",
	}

	if normalized, exists := typeMap[dataType]; exists {
		return normalized
	}
	return dataType
}
