package main

import (
	"os"

	"github.com/anuryadi/ditto/cmd"
	"github.com/anuryadi/ditto/pkg/logger"
)

// Version information (set by ldflags during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Initialize logger
	logger.Init(false) // verbose mode off by default

	// Set version info for cmd package
	cmd.SetVersionInfo(version, commit, date)

	// Execute root command
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
