package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

var (
	// Version info
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"

	// Global flags
	configFile string
	dataDir    string
	logLevel   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "v2v-node",
		Short: "V2V Blockchain Node",
		Long: `V2V Blockchain Node - A lightweight blockchain for vehicle platooning.

This node supports:
  - PBFT consensus for platoon management
  - Vehicle identity authentication
  - V2V message verification
  - State traceability and audit logging`,
		Version: fmt.Sprintf("%s (build: %s, commit: %s)", version, buildTime, gitCommit),
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data-dir", "d", "./data", "data directory")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log level (debug|info|warn|error)")

	// Add commands
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newIdentityCmd())
	rootCmd.AddCommand(newPlatoonCmd())
	rootCmd.AddCommand(newQueryCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", logger.ErrField(err))
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("V2V Blockchain Node\n")
			fmt.Printf("Version:    %s\n", version)
			fmt.Printf("Build Time: %s\n", buildTime)
			fmt.Printf("Git Commit: %s\n", gitCommit)
		},
	}
}
