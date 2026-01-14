package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/anuryadi/ditto/internal/config"
	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/internal/sync"
	"github.com/anuryadi/ditto/internal/ui"
	"github.com/anuryadi/ditto/pkg/logger"
)

var (
	strategyFlag string
	dryRunFlag   bool
	forceFlag    bool
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize data from source to target database",
	Long: `Synchronize data tables from a source database to a target database.

Supports two strategies:
1. full: Truncates target table and copies all data from source.
2. incremental: Copies new rows based on primary key or timestamp column.

Examples:
  # Full sync of specific tables
  ditto sync --source "postgres://..." --target "mysql://..." --tables users,orders --strategy full

  # Incremental sync (requires primary_key configuration)
  ditto sync --source "..." --target "..." --tables orders --strategy incremental

  # Dry run to see what would happen
  ditto sync --source "..." --target "..." --dry-run`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().StringVarP(&sourceFlag, "source", "s", "", "Source database connection string (required)")
	syncCmd.Flags().StringVarP(&targetFlag, "target", "t", "", "Target database connection string (required)")
	syncCmd.Flags().StringSliceVar(&tablesFlag, "tables", nil, "Specific tables to sync (comma-separated)")
	syncCmd.Flags().StringVar(&strategyFlag, "strategy", "full", "Sync strategy: full, incremental")
	syncCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Perform a dry run without modifying data")
	syncCmd.Flags().BoolVar(&forceFlag, "force", false, "Force execution without confirmation (for full sync)")

	syncCmd.MarkFlagRequired("source")
	syncCmd.MarkFlagRequired("target")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// 1. Parse configuration
	sourceCfg, err := config.ParseDSN(sourceFlag)
	if err != nil {
		return fmt.Errorf("invalid source connection string: %w", err)
	}

	targetCfg, err := config.ParseDSN(targetFlag)
	if err != nil {
		return fmt.Errorf("invalid target connection string: %w", err)
	}

	// 2. Safety check for full sync
	if strategyFlag == "full" && !dryRunFlag && !forceFlag {
		if !confirmFullSync() {
			return fmt.Errorf("operation cancelled by user")
		}
	}

	// 3. Connect to databases
	spinner, _ := ui.NewSpinner("Connecting to databases...")

	sourceDriver, err := database.NewDriver(sourceCfg)
	if err != nil {
		spinner.Fail("Failed to create source driver")
		return err
	}
	if err := sourceDriver.Connect(ctx); err != nil {
		spinner.Fail("Failed to connect to source database")
		return err
	}
	defer sourceDriver.Close()

	targetDriver, err := database.NewDriver(targetCfg)
	if err != nil {
		spinner.Fail("Failed to create target driver")
		return err
	}
	if err := targetDriver.Connect(ctx); err != nil {
		spinner.Fail("Failed to connect to target database")
		return err
	}
	defer targetDriver.Close()

	spinner.Success("Connected to databases")

	// 4. Prepare Table Configs
	var tableConfigs []config.TableConfig

	// If tables passed via flag, use them
	if len(tablesFlag) > 0 {
		for _, tableName := range tablesFlag {
			// For CLI usage, we use default/inferred primary keys if not specified in config file
			// But SyncEngine requires config.TableConfig which might need PrimaryKey for incremental.
			// Ideally we should merge flags with config file if present.
			// For this MVP CLI usage, we try to auto-detect or use simple defaults.

			// Try to find table config in Viper (loaded from file) if exists
			var tc config.TableConfig
			found := false

			// Hacky way to lookup from loaded config tables
			var cfg config.Config
			if err := viper.Unmarshal(&cfg); err == nil {
				for _, t := range cfg.Sync.Tables {
					if t.Name == tableName {
						tc = t
						found = true
						break
					}
				}
			}

			if !found {
				tc = config.TableConfig{Name: tableName}
				// Auto-detect PK if incremental?
				if strategyFlag == "incremental" {
					pk, err := targetDriver.GetPrimaryKey(ctx, tableName)
					if err == nil && len(pk) > 0 {
						tc.PrimaryKey = pk[0] // Use first column of PK
						logger.Infof("Auto-detected primary key for %s: %s", tableName, tc.PrimaryKey)
					} else {
						logger.Warnf("Could not auto-detect primary key for %s, incremental sync might fail if not configured", tableName)
					}
				}
			}
			tableConfigs = append(tableConfigs, tc)
		}
	} else {
		// Use tables from config file
		var cfg config.Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		tableConfigs = cfg.Sync.Tables
		if len(tableConfigs) == 0 {
			return fmt.Errorf("no tables specified to sync. Use --tables flag or config file")
		}
	}

	// 5. Initialize Engine
	syncCfg := config.SyncConfig{
		Strategy:  strategyFlag,
		BatchSize: 1000,
		Tables:    tableConfigs,
	}

	engine := sync.NewEngine(sourceDriver, targetDriver, syncCfg)

	// 6. Run Sync
	if err := engine.Run(ctx, dryRunFlag); err != nil {
		return err
	}

	return nil
}

func confirmFullSync() bool {
	fmt.Println("WARNING: Full sync strategy is selected.")
	fmt.Println("This will TRUNCATE (delete all data) from the target tables before syncing.")
	fmt.Print("Are you sure you want to continue? [y/N]: ")

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
