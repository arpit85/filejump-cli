package cmd

import (
	"fmt"
	"os"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/arpit85/filejump-cli/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "filejump",
	Short: "FileJump command-line client",
	Long:  "FileJump CLI — manage your files and folders from the terminal.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadConfig reads the persisted config; exits with a friendly message if not logged in.
func loadConfig() *config.Config {
	c, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}
	return c
}

// requireClient returns an authenticated client, exiting if not logged in.
func requireClient() (*api.Client, *config.Config) {
	cfg := loadConfig()
	if cfg.Token == "" || cfg.Server == "" {
		fmt.Fprintln(os.Stderr, "Not logged in. Run `filejump login` first.")
		os.Exit(1)
	}
	return api.New(cfg.Server, cfg.Token), cfg
}
