package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/arpit85/filejump-cli/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "filejump",
	Short: "FileJump command-line client",
	Long:  "FileJump CLI — manage your files and folders from the terminal.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("workspace") {
			id, err := strconv.Atoi(flagWorkspace)
			if err != nil {
				return fmt.Errorf("--workspace must be an integer ID (got %q)", flagWorkspace)
			}
			if id > 0 {
				workspaceOverride = &id
			} else {
				workspaceOverride = &personalWorkspace // explicit personal
			}
		}
		return nil
	},
}

// flagWorkspace holds the raw --workspace value; workspaceOverride is the
// parsed pointer used by requireClient. A non-nil override always wins.
var (
	flagWorkspace      string
	workspaceOverride  *int
	personalWorkspace  int // sentinel address target meaning "personal space"
)

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
	client := api.New(cfg.Server, cfg.Token)
	switch {
	case workspaceOverride != nil && workspaceOverride != &personalWorkspace:
		client.WorkspaceID = workspaceOverride
	case workspaceOverride == &personalWorkspace:
		client.WorkspaceID = nil
	default:
		if cfg.WorkspaceID != 0 {
			wid := cfg.WorkspaceID
			client.WorkspaceID = &wid
		}
	}
	return client, cfg
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagWorkspace, "workspace", "w", "", "workspace ID to operate in (0 = personal space; overrides the saved active workspace)")
}
