package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "List and switch the active workspace",
	Long: `Manage which workspace the CLI operates in.

FileJump scopes every file and folder under either your personal space or a
workspace. The active workspace is saved in your config and used by all data
commands (ls, upload, download, mkdir, mv, rm, sync). Use "workspace use" to
switch, or pass --workspace <id> to any command for a one-off override.`,
}

var workspaceLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List workspaces you own or belong to",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg := requireClient()
		ws, err := client.ListWorkspaces()
		if err != nil {
			return err
		}
		if len(ws) == 0 {
			fmt.Println("No workspaces. You are using your personal space.")
			return nil
		}
		sort.Slice(ws, func(i, j int) bool { return ws[i].ID < ws[j].ID })

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tROLE\tACTIVE")
		// Personal space row first for context.
		personalMarker := ""
		if cfg.WorkspaceID == 0 {
			personalMarker = "*"
		}
		fmt.Fprintf(w, "-\t(personal)\towner\t%s\n", personalMarker)
		for _, wk := range ws {
			marker := ""
			if cfg.WorkspaceID == wk.ID {
				marker = "*"
			}
			role := wk.MyRole
			if role == "" {
				role = "-"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", wk.ID, wk.Name, role, marker)
		}
		return w.Flush()
	},
}

var workspaceUseCmd = &cobra.Command{
	Use:   "use <id-or-name>",
	Short: "Switch the active workspace (use \"personal\" or 0 for your personal space)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg := requireClient()
		arg := strings.TrimSpace(args[0])

		// Personal space shortcuts.
		if arg == "0" || strings.EqualFold(arg, "personal") {
			cfg.WorkspaceID = 0
			cfg.WorkspaceName = ""
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Println("Switched to your personal space.")
			return nil
		}

		// Numeric ID takes precedence.
		if id, err := strconv.Atoi(arg); err == nil {
			ws, err := client.ListWorkspaces()
			if err != nil {
				return err
			}
			for _, wk := range ws {
				if wk.ID == id {
					cfg.WorkspaceID = wk.ID
					cfg.WorkspaceName = wk.Name
					if err := cfg.Save(); err != nil {
						return err
					}
					fmt.Printf("Switched to workspace %q (id %d).\n", wk.Name, wk.ID)
					return nil
				}
			}
			return fmt.Errorf("no workspace with id %d (run `filejump workspace ls`)", id)
		}

		// Match by name (case-insensitive, exact).
		ws, err := client.ListWorkspaces()
		if err != nil {
			return err
		}
		var foundID int
		var foundName string
		var count int
		lower := strings.ToLower(arg)
		for i := range ws {
			if strings.ToLower(ws[i].Name) == lower {
				count++
				if count == 1 {
					foundID = ws[i].ID
					foundName = ws[i].Name
				}
			}
		}
		if count == 0 {
			return fmt.Errorf("no workspace named %q (run `filejump workspace ls`)", arg)
		}
		if count > 1 {
			return fmt.Errorf("multiple workspaces named %q; switch by numeric id instead", arg)
		}
		cfg.WorkspaceID = foundID
		cfg.WorkspaceName = foundName
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Switched to workspace %q (id %d).\n", foundName, foundID)
		return nil
	},
}

var workspaceCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active workspace",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		if cfg.Token == "" {
			fmt.Fprintln(os.Stderr, "Not logged in. Run `filejump login` first.")
			os.Exit(1)
		}
		if cfg.WorkspaceID == 0 {
			fmt.Println("Active workspace: personal space")
		} else if cfg.WorkspaceName != "" {
			fmt.Printf("Active workspace: %s (id %d)\n", cfg.WorkspaceName, cfg.WorkspaceID)
		} else {
			fmt.Printf("Active workspace: id %d\n", cfg.WorkspaceID)
		}
		return nil
	},
}

var workspaceResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Switch back to your personal space",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		if cfg.Token == "" {
			fmt.Fprintln(os.Stderr, "Not logged in. Run `filejump login` first.")
			os.Exit(1)
		}
		cfg.WorkspaceID = 0
		cfg.WorkspaceName = ""
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Println("Switched to your personal space.")
		return nil
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceLsCmd, workspaceUseCmd, workspaceCurrentCmd, workspaceResetCmd)
	rootCmd.AddCommand(workspaceCmd)
}
