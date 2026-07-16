package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/arpit85/filejump-cli/internal/api"
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

var workspaceDeleteYes bool

var workspaceRenameCmd = &cobra.Command{
	Use:   "rename <id|name> <new-name>",
	Short: "Rename a workspace you own or administer",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg := requireClient()
		wk, err := resolveWorkspace(client, args[0])
		if err != nil {
			return err
		}
		newName := strings.TrimSpace(args[1])
		if newName == "" {
			return fmt.Errorf("new name must not be empty")
		}
		updated, err := client.UpdateWorkspace(wk.ID, newName)
		if err != nil {
			return err
		}
		// Keep the saved active-workspace label in sync if this was active.
		if cfg.WorkspaceID == wk.ID {
			cfg.WorkspaceName = updated
			if err := cfg.Save(); err != nil {
				return err
			}
		}
		fmt.Printf("Renamed workspace id %d to %q.\n", wk.ID, updated)
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <id|name>",
	Short: "Permanently delete a workspace you own (and all its files/folders/members)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg := requireClient()
		wk, err := resolveWorkspace(client, args[0])
		if err != nil {
			return err
		}
		if !workspaceDeleteYes {
			fmt.Printf("Permanently delete workspace %q (id %d)? This deletes ALL its files, folders, members, and invitations. [y/N] ", wk.Name, wk.ID)
			var resp string
			fmt.Scanln(&resp)
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(resp)), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}
		if err := client.DeleteWorkspace(wk.ID); err != nil {
			return err
		}
		// If the deleted workspace was the active one, fall back to personal space.
		if cfg.WorkspaceID == wk.ID {
			cfg.WorkspaceID = 0
			cfg.WorkspaceName = ""
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Println("Deleted. Active workspace cleared — now using your personal space.")
			return nil
		}
		fmt.Printf("Deleted workspace %q (id %d).\n", wk.Name, wk.ID)
		return nil
	},
}

// resolveWorkspace finds a workspace by numeric id or (case-insensitive, exact)
// name. Names must match uniquely; otherwise the caller is asked to use the id.
func resolveWorkspace(client apiClientLike, arg string) (api.Workspace, error) {
	ws, err := client.ListWorkspaces()
	if err != nil {
		return api.Workspace{}, err
	}
	if id, err := strconv.Atoi(arg); err == nil {
		for _, wk := range ws {
			if wk.ID == id {
				return wk, nil
			}
		}
		return api.Workspace{}, fmt.Errorf("no workspace with id %d (run `filejump workspace ls`)", id)
	}
	lower := strings.ToLower(strings.TrimSpace(arg))
	var match api.Workspace
	count := 0
	for _, wk := range ws {
		if strings.ToLower(wk.Name) == lower {
			match = wk
			count++
		}
	}
	switch count {
	case 0:
		return api.Workspace{}, fmt.Errorf("no workspace named %q (run `filejump workspace ls`)", arg)
	case 1:
		return match, nil
	default:
		return api.Workspace{}, fmt.Errorf("multiple workspaces named %q; specify by numeric id instead", arg)
	}
}

// apiClientLike is the minimal surface resolveWorkspace needs. The real
// *api.Client satisfies it.
type apiClientLike interface {
	ListWorkspaces() ([]api.Workspace, error)
}

func init() {
	workspaceCmd.AddCommand(workspaceLsCmd, workspaceUseCmd, workspaceCurrentCmd, workspaceResetCmd, workspaceRenameCmd, workspaceDeleteCmd)
	workspaceDeleteCmd.Flags().BoolVarP(&workspaceDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	rootCmd.AddCommand(workspaceCmd)
}
