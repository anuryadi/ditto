package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/anuryadi/ditto/internal/config"
	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/internal/export"
	"github.com/anuryadi/ditto/internal/ui"
	"github.com/anuryadi/ditto/pkg/logger"
)

var (
	tableFlag string
	queryFlag string
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export table data to a file",
	Long: `Export data from a database table to various formats.

Supported formats:
- json: JSON array with metadata
- csv: Comma-separated values  
- sql: SQL INSERT statements

Examples:
  # Export to JSON
  ditto export --source "postgres://..." --table users --format json --output users.json

  # Export to CSV
  ditto export --source "mysql://..." --table orders --format csv --output orders.csv

  # Export to SQL
  ditto export --source "postgres://..." --table products --format sql --output products.sql

  # Custom query export
  ditto export --source "postgres://..." --query "SELECT id,name FROM users WHERE active=true" --format json`,
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&sourceFlag, "source", "s", "", "Source database connection string (required)")
	exportCmd.Flags().StringVar(&tableFlag, "table", "", "Table name to export")
	exportCmd.Flags().StringVar(&queryFlag, "query", "", "Custom SQL query (overrides --table)")
	exportCmd.Flags().StringVarP(&formatFlag, "format", "f", "json", "Output format: json, csv, sql")
	exportCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path (default: stdout)")

	exportCmd.MarkFlagRequired("source")
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Validate flags
	if tableFlag == "" && queryFlag == "" {
		return fmt.Errorf("either --table or --query is required")
	}

	// Parse source connection
	sourceCfg, err := config.ParseDSN(sourceFlag)
	if err != nil {
		return fmt.Errorf("invalid source connection string: %w", err)
	}

	// Connect to database
	spinner, _ := ui.NewSpinner("Connecting to database...")

	driver, err := database.NewDriver(sourceCfg)
	if err != nil {
		spinner.Fail("Failed to create driver")
		return err
	}
	if err := driver.Connect(ctx); err != nil {
		spinner.Fail("Failed to connect to database")
		return err
	}
	defer driver.Close()
	spinner.Success("Connected to database")

	// Determine output
	var output *os.File
	if outputFlag != "" {
		output, err = os.Create(outputFlag)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}

	// Prepare export options
	tableName := tableFlag
	if tableName == "" && queryFlag != "" {
		tableName = "query_result"
	}

	options := export.Options{
		Format:    export.Format(formatFlag),
		TableName: tableName,
		Query:     queryFlag,
	}

	// Perform export
	spinner, _ = ui.NewSpinner(fmt.Sprintf("Exporting %s...", tableName))

	exporter := export.NewExporter(driver, options, output)
	rowCount, err := exporter.Export(ctx)
	if err != nil {
		spinner.Fail("Export failed")
		return err
	}

	spinner.Success(fmt.Sprintf("Exported %d rows", rowCount))

	if outputFlag != "" {
		logger.Infof("Data saved to: %s", outputFlag)
	}

	return nil
}
