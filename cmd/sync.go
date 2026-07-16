package cmd

import (
	"fmt"
	"os"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/arpit85/filejump-cli/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncRemoteRoot string
	syncDryRun     bool
	syncNoPush     bool
	syncNoPull     bool
)

// syncClient is set by the sync command so the upload callback can reach it.
var syncClient *api.Client

var syncCmd = &cobra.Command{
	Use:   "sync <local-dir> [remote-root]",
	Short: "Two-way sync a local directory with a remote folder",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		syncClient = client

		localDir := args[0]
		remoteRoot := "/"
		if len(args) > 1 {
			remoteRoot = args[1]
		}

		info, err := os.Stat(localDir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", localDir)
		}

		state, err := sync.LoadState(localDir, remoteRoot)
		if err != nil {
			return err
		}

		if !syncNoPull {
			fmt.Println("Pulling remote changes...")
			n, err := sync.Pull(client, state, localDir)
			if err != nil {
				return err
			}
			fmt.Printf("Pulled %d change(s).\n", n)
			if err := state.Save(); err != nil {
				return err
			}
		}

		if !syncNoPush {
			fmt.Println("Pushing local changes...")
			n, err := sync.Push(client, state, localDir, syncUpload)
			if err != nil {
				return err
			}
			fmt.Printf("Pushed %d change(s).\n", n)
			if err := state.Save(); err != nil {
				return err
			}
		}

		fmt.Println("Sync complete.")
		return nil
	},
}

// syncUpload is the callback passed to sync.Push: it ensures the remote folder
// exists, then uploads the local file into it using the best strategy.
func syncUpload(localPath, remoteFolder string) (*api.File, error) {
	resolver := pathresolve.New(syncClient)
	folderID, err := resolver.EnsureFolder(remoteFolder)
	if err != nil {
		return nil, err
	}
	return uploadBest(syncClient, localPath, folderID)
}

func init() {
	syncCmd.Flags().StringVarP(&syncRemoteRoot, "remote", "r", "", "remote root path (default /)")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "show what would change without applying")
	syncCmd.Flags().BoolVar(&syncNoPush, "no-push", false, "skip pushing local changes")
	syncCmd.Flags().BoolVar(&syncNoPull, "no-pull", false, "skip pulling remote changes")
	rootCmd.AddCommand(syncCmd)
}
