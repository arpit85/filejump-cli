package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download <remote-path> [local-path]",
	Short: "Download a remote file to a local path",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		resolver := pathresolve.New(client)
		file, err := resolver.ResolveFile(args[0])
		if err != nil {
			return err
		}
		dest := filepath.Base(file.Name)
		if len(args) > 1 {
			dest = args[1]
		}
		fmt.Printf("Downloading %s (%s) ... ", file.Name, humanSize(file.Size))
		if err := client.Download(file.ID, dest); err != nil {
			fmt.Println("failed")
			return err
		}
		fmt.Println("done")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
