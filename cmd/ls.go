package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var lsLong bool

var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List folders and files at a path (default: root)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		p := ""
		if len(args) > 0 {
			p = args[0]
		}
		resolver := pathresolve.New(client)
		folderID, err := resolver.ResolveFolder(p)
		if err != nil {
			return err
		}
		folders, err := client.ListFolders(folderID)
		if err != nil {
			return err
		}
		files, err := client.ListFiles(folderID)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		for _, f := range folders {
			if lsLong {
				fmt.Fprintf(w, "d\t-\t\t%s\t%s\n", f.Name, f.Path)
			} else {
				fmt.Fprintf(w, "%s/\n", f.Name)
			}
		}
		for _, f := range files {
			if lsLong {
				fmt.Fprintf(w, "f\t%s\t\t%s\n", humanSize(f.Size), f.Name)
			} else {
				fmt.Fprintf(w, "%s\n", f.Name)
			}
		}
		return w.Flush()
	},
}

func init() {
	lsCmd.Flags().BoolVarP(&lsLong, "long", "l", false, "long listing")
	rootCmd.AddCommand(lsCmd)
}
