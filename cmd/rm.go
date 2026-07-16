package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var rmForce bool

var rmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Delete a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		resolver := pathresolve.New(client)
		file, err := resolver.ResolveFile(args[0])
		if err != nil {
			return err
		}
		if !rmForce {
			fmt.Printf("Delete %s (%s)? [y/N] ", file.Name, humanSize(file.Size))
			reader := bufio.NewReader(os.Stdin)
			resp, _ := reader.ReadString('\n')
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(resp)), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}
		return client.DeleteFile(file.ID)
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "skip confirmation")
	rootCmd.AddCommand(rmCmd)
}
