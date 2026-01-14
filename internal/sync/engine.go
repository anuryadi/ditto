package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/anuryadi/ditto/internal/config"
	"github.com/anuryadi/ditto/internal/database"
	"github.com/anuryadi/ditto/pkg/logger"
)

// Engine handles the synchronization process
type Engine struct {
	source    database.Driver
	target    database.Driver
	config    config.SyncConfig
	batchSize int
}

// NewEngine creates a new synchronization engine
func NewEngine(source, target database.Driver, cfg config.SyncConfig) *Engine {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	return &Engine{
		source:    source,
		target:    target,
		config:    cfg,
		batchSize: batchSize,
	}
}

// Run executes the synchronization based on configuration
func (e *Engine) Run(ctx context.Context, dryRun bool) error {
	logger.Info("Starting synchronization...")

	startTime := time.Now()
	totalRows := 0

	for _, tableCfg := range e.config.Tables {
		// Determine strategy
		strategyName := e.config.Strategy

		var strategy Strategy
		switch strategyName {
		case "full":
			strategy = &FullStrategy{BatchSize: e.batchSize}
		case "incremental":
			strategy = &IncrementalStrategy{
				BatchSize:       e.batchSize,
				PrimaryKey:      tableCfg.PrimaryKey,
				TimestampColumn: tableCfg.TimestampColumn,
			}
		default:
			return fmt.Errorf("unknown sync strategy: %s", strategyName)
		}

		logger.Infof("Syncing table '%s' using %s strategy...", tableCfg.Name, strategyName)

		if dryRun {
			logger.Infof("[DRY RUN] Would sync table %s", tableCfg.Name)
			continue
		}

		rowsSynced, err := strategy.Execute(ctx, e.source, e.target, tableCfg.Name)
		if err != nil {
			return fmt.Errorf("failed to sync table %s: %w", tableCfg.Name, err)
		}

		totalRows += rowsSynced
		logger.Infof("Synced %d rows for table %s", rowsSynced, tableCfg.Name)
	}

	duration := time.Since(startTime)
	logger.Infof("Synchronization completed in %s. Total rows: %d", duration, totalRows)

	return nil
}
