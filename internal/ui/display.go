package ui

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"

	"github.com/anuryadi/ditto/internal/schema"
)

// PrintCompareResult prints the comparison result in a formatted table
func PrintCompareResult(result *schema.CompareResult) {
	// Print header
	pterm.DefaultHeader.WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("Schema Comparison Result")

	fmt.Println()

	// Print database info
	pterm.Info.Printfln("Source: %s", result.SourceDB)
	pterm.Info.Printfln("Target: %s", result.TargetDB)
	fmt.Println()

	if len(result.Differences) == 0 {
		pterm.Success.Println("No differences found! Schemas are identical.")
		return
	}

	// Group differences by type
	tables := []schema.Difference{}
	columns := []schema.Difference{}
	indexes := []schema.Difference{}

	for _, diff := range result.Differences {
		switch diff.ObjectType {
		case "table":
			tables = append(tables, diff)
		case "column":
			columns = append(columns, diff)
		case "index":
			indexes = append(indexes, diff)
		}
	}

	// Print table differences
	if len(tables) > 0 {
		printDifferenceSection("Tables", tables)
	}

	// Print column differences
	if len(columns) > 0 {
		printDifferenceSection("Columns", columns)
	}

	// Print index differences
	if len(indexes) > 0 {
		printDifferenceSection("Indexes", indexes)
	}

	// Print summary
	printSummary(result.Summary)
}

func printDifferenceSection(title string, diffs []schema.Difference) {
	pterm.DefaultSection.Println(title)

	tableData := pterm.TableData{
		{"Status", "Object", "Description"},
	}

	for _, diff := range diffs {
		status := getStatusIcon(diff.Type)
		name := diff.ObjectName
		if diff.TableName != "" {
			name = diff.TableName + "." + diff.ObjectName
		}
		tableData = append(tableData, []string{status, name, truncateString(diff.Description, 60)})
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	fmt.Println()
}

func getStatusIcon(diffType schema.DifferenceType) string {
	switch diffType {
	case schema.DiffAdded:
		return pterm.Green("+ ADDED")
	case schema.DiffRemoved:
		return pterm.Red("- REMOVED")
	case schema.DiffModified:
		return pterm.Yellow("~ MODIFIED")
	default:
		return string(diffType)
	}
}

func printSummary(summary schema.Summary) {
	pterm.DefaultSection.Println("Summary")

	data := [][]string{
		{"Tables Added", fmt.Sprintf("%d", summary.TablesAdded)},
		{"Tables Removed", fmt.Sprintf("%d", summary.TablesRemoved)},
		{"Columns Added", fmt.Sprintf("%d", summary.ColumnsAdded)},
		{"Columns Removed", fmt.Sprintf("%d", summary.ColumnsRemoved)},
		{"Columns Changed", fmt.Sprintf("%d", summary.ColumnsChanged)},
		{"Indexes Added", fmt.Sprintf("%d", summary.IndexesAdded)},
		{"Indexes Removed", fmt.Sprintf("%d", summary.IndexesRemoved)},
		{"", ""},
		{"Total Differences", pterm.Bold.Sprintf("%d", summary.TotalDiffs)},
	}

	for _, row := range data {
		if row[0] == "" {
			continue
		}
		fmt.Printf("  %-20s %s\n", row[0]+":", row[1])
	}
	fmt.Println()
}

func truncateString(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintTables prints a list of tables
func PrintTables(tables []string, dbName string) {
	pterm.DefaultHeader.WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite)).
		Printf("Tables in %s", dbName)

	fmt.Println()

	for i, table := range tables {
		fmt.Printf("  %3d. %s\n", i+1, table)
	}
	fmt.Printf("\n  Total: %d tables\n", len(tables))
}

// Spinner creates a new spinner
func NewSpinner(message string) (*pterm.SpinnerPrinter, error) {
	return pterm.DefaultSpinner.Start(message)
}

// ProgressBar creates a new progress bar
func NewProgressBar(total int, title string) (*pterm.ProgressbarPrinter, error) {
	return pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithRemoveWhenDone().
		Start()
}
