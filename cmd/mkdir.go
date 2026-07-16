package cmd

import (
	"fmt"

	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var mkdirParents bool

var mkdirCmd = &cobra.Command{
	Use:   "mkdir <path>",
	Short: "Create a folder (use -p to create parents)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		resolver := pathresolve.New(client)
		if mkdirParents {
			_, err := resolver.EnsureFolder(args[0])
			return err
		}
		// Single-level create: resolve parent, then create leaf.
		dir, leaf := splitDir(args[0])
		if leaf == "" {
			return fmt.Errorf("invalid folder path")
		}
		parentID, err := resolver.ResolveFolder(dir)
		if err != nil {
			return err
		}
		_, err = client.CreateFolder(leaf, parentID)
		return err
	},
}

func init() {
	mkdirCmd.Flags().BoolVarP(&mkdirParents, "parents", "p", false, "create parent folders as needed")
	rootCmd.AddCommand(mkdirCmd)
}
