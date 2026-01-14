package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/anuryadi/ditto/internal/config"
	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/internal/schema"
	"github.com/anuryadi/ditto/internal/ui"
	"github.com/anuryadi/ditto/pkg/logger"
)

var (
	sourceFlag string
	targetFlag string
	outputFlag string
	formatFlag string
	tablesFlag []string
)

// compareCmd represents the compare command
var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare schemas between two databases",
	Long: `Compare the schema structure between a source and target database.

This command analyzes tables, columns, indexes, and constraints to identify
differences between two databases. Useful for migrations, audits, and
ensuring consistency across environments.

Examples:
  # Compare PostgreSQL to MySQL
  ditto compare --source "postgres://user:pass@localhost/db1" --target "mysql://user:pass@localhost/db2"

  # Output differences to JSON file
  ditto compare --source "postgres://..." --target "mysql://..." --output diff.json --format json

  # Compare specific tables only
  ditto compare --source "postgres://..." --target "mysql://..." --tables users,orders`,
	RunE: runCompare,
}

func init() {
	rootCmd.AddCommand(compareCmd)

	compareCmd.Flags().StringVarP(&sourceFlag, "source", "s", "", "Source database connection string (required)")
	compareCmd.Flags().StringVarP(&targetFlag, "target", "t", "", "Target database connection string (required)")
	compareCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path (optional)")
	compareCmd.Flags().StringVarP(&formatFlag, "format", "f", "table", "Output format: table, json (default: table)")
	compareCmd.Flags().StringSliceVar(&tablesFlag, "tables", nil, "Specific tables to compare (comma-separated)")

	compareCmd.MarkFlagRequired("source")
	compareCmd.MarkFlagRequired("target")
}

func runCompare(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Parse connection strings
	sourceCfg, err := config.ParseDSN(sourceFlag)
	if err != nil {
		return fmt.Errorf("invalid source connection string: %w", err)
	}

	targetCfg, err := config.ParseDSN(targetFlag)
	if err != nil {
		return fmt.Errorf("invalid target connection string: %w", err)
	}

	// Create database drivers
	sourceDriver, err := database.NewDriver(sourceCfg)
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	targetDriver, err := database.NewDriver(targetCfg)
	if err != nil {
		return fmt.Errorf("failed to create target driver: %w", err)
	}

	// Connect to databases
	spinner, _ := ui.NewSpinner("Connecting to source database...")
	if err := sourceDriver.Connect(ctx); err != nil {
		spinner.Fail("Failed to connect to source database")
		return err
	}
	defer sourceDriver.Close()
	spinner.Success("Connected to source database")

	spinner, _ = ui.NewSpinner("Connecting to target database...")
	if err := targetDriver.Connect(ctx); err != nil {
		spinner.Fail("Failed to connect to target database")
		return err
	}
	defer targetDriver.Close()
	spinner.Success("Connected to target database")

	// Perform comparison
	spinner, _ = ui.NewSpinner("Comparing schemas...")
	comparator := schema.NewComparator(sourceDriver, targetDriver)
	result, err := comparator.Compare(ctx)
	if err != nil {
		spinner.Fail("Schema comparison failed")
		return fmt.Errorf("comparison failed: %w", err)
	}
	spinner.Success("Schema comparison complete")

	fmt.Println()

	// Output results
	switch formatFlag {
	case "json":
		return outputJSON(result)
	default:
		ui.PrintCompareResult(result)
	}

	// Save to file if output flag is set
	if outputFlag != "" {
		if err := saveToFile(result); err != nil {
			logger.Errorf("Failed to save output: %v", err)
		}
	}

	return nil
}

func outputJSON(result *schema.CompareResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func saveToFile(result *schema.CompareResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(outputFlag, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Infof("Results saved to: %s", outputFlag)
	return nil
}
