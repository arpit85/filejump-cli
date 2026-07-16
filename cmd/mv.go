package cmd

import (
	"fmt"
	"path"
	"strings"

	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var mvCmd = &cobra.Command{
	Use:   "mv <src> <dest>",
	Short: "Move or rename a file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		resolver := pathresolve.New(client)
		file, err := resolver.ResolveFile(args[0])
		if err != nil {
			return err
		}

		dest := args[1]

		// Dest ends with "/" -> move into that folder, keep current name.
		if strings.HasSuffix(dest, "/") {
			folderID, err := resolver.ResolveFolder(strings.TrimSuffix(dest, "/"))
			if err != nil {
				return err
			}
			return client.MoveFile(file.ID, folderID)
		}

		// Dest names an existing folder -> move into it, keep current name.
		if folderID, err := resolver.ResolveFolder(dest); err == nil {
			return client.MoveFile(file.ID, folderID)
		}

		// Otherwise treat dest as parent-folder/new-name.
		dir, name := splitDir(dest)
		if name == "" {
			return fmt.Errorf("invalid destination path")
		}
		folderID, err := resolver.ResolveFolder(dir)
		if err != nil {
			return err
		}

		// Move to the parent folder (harmless if already there), then rename if needed.
		if err := client.MoveFile(file.ID, folderID); err != nil {
			return err
		}
		if name != file.Name {
			return client.RenameFile(file.ID, name)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mvCmd)
}

// splitDir splits /a/b/c into ("/a/b", "c").
func splitDir(p string) (string, string) {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	clean := path.Clean(p)
	return path.Split(clean)
}
