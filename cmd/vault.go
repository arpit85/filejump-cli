package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	vaultPassword     string
	vaultNewPassword  string
	vaultDescription  string
	vaultIcon         string
	vaultAutoLock     bool
	vaultLockTimeout  int
	vaultKeepUnlocked bool
	vaultDeleteYes    bool
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage encrypted vaults and the files inside them",
	Long: `Manage encrypted vaults.

A vault is a private, password-locked collection of your own files. There is no
"upload directly into a vault" endpoint, so 'vault upload' uploads each file to
your personal space first and then adds it to the vault (unlocking it with the
vault password). The vault password is never stored; supply it per command via
--password or an interactive prompt.`,
}

var vaultLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List your vaults",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		vaults, err := client.ListVaults()
		if err != nil {
			return err
		}
		if len(vaults) == 0 {
			fmt.Println("No vaults. Create one with `filejump vault create <name>`.")
			return nil
		}
		sort.Slice(vaults, func(i, j int) bool { return vaults[i].ID < vaults[j].ID })
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tFILES\tSIZE\tAUTO-LOCK\tTIMEOUT(min)")
		for _, v := range vaults {
			fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%v\t%d\n", v.ID, v.Name, v.FilesCount, humanSize(v.TotalSize), v.AutoLock, v.LockTimeout)
		}
		return w.Flush()
	},
}

var vaultShowCmd = &cobra.Command{
	Use:   "show <vault>",
	Short: "Show vault details and lock status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Vault:     %q (id %d)\n", v.Name, v.ID)
		if v.Description != "" {
			fmt.Printf("Description: %s\n", v.Description)
		}
		fmt.Printf("Files:     %d\n", v.FilesCount)
		fmt.Printf("Size:      %s\n", humanSize(v.TotalSize))
		fmt.Printf("Auto-lock: %v (timeout %d min)\n", v.AutoLock, v.LockTimeout)
		if v.CreatedAt != "" {
			fmt.Printf("Created:   %s\n", v.CreatedAt)
		}
		if unlocked, exp, err := client.VaultStatus(v.ID); err == nil {
			if unlocked {
				fmt.Println("Status:    unlocked")
				if exp != "" {
					fmt.Printf("           unlock expires: %s\n", exp)
				}
			} else {
				fmt.Println("Status:    locked")
			}
		}
		return nil
	},
}

var vaultCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new vault (prompts for a password)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		pw, err := vaultCreatePassword()
		if err != nil {
			return err
		}
		opts := api.VaultOptions{Description: vaultDescription, Icon: vaultIcon}
		if cmd.Flags().Changed("auto-lock") {
			b := vaultAutoLock
			opts.AutoLock = &b
		}
		if vaultLockTimeout > 0 {
			opts.LockTimeout = vaultLockTimeout
		}
		v, err := client.CreateVault(args[0], pw, opts)
		if err != nil {
			return err
		}
		fmt.Printf("Created vault %q (id %d).\n", v.Name, v.ID)
		return nil
	},
}

var vaultUploadCmd = &cobra.Command{
	Use:   "upload <vault> <local-files...>",
	Short: "Upload local files and add them to a vault",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		token, err := unlockVault(client, v)
		if err != nil {
			return err
		}

		// Vault files are private; force personal space so vault-destined files
		// never land in a shared workspace.
		saved := client.WorkspaceID
		client.WorkspaceID = nil
		var fileIDs []int
		for _, local := range args[1:] {
			fmt.Printf("Uploading %s ... ", filepath.Base(local))
			f, err := uploadBest(client, local, nil)
			if err != nil {
				fmt.Println("failed")
				client.WorkspaceID = saved
				return err
			}
			fmt.Printf("done (%s)\n", humanSize(f.Size))
			fileIDs = append(fileIDs, f.ID)
		}
		client.WorkspaceID = saved

		added, total, err := client.AddVaultFiles(v.ID, token, fileIDs, nil)
		if err != nil {
			return err
		}
		maybeLockVault(client, v)
		fmt.Printf("Added %d file(s) to vault %q (total %d).\n", added, v.Name, total)
		return nil
	},
}

var vaultAddCmd = &cobra.Command{
	Use:   "add <vault> <remote-paths...>",
	Short: "Add already-uploaded files to a vault",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		token, err := unlockVault(client, v)
		if err != nil {
			return err
		}
		var fileIDs []int
		for _, p := range args[1:] {
			f, err := resolveAnyFile(client, p)
			if err != nil {
				return err
			}
			fileIDs = append(fileIDs, f.ID)
		}
		added, total, err := client.AddVaultFiles(v.ID, token, fileIDs, nil)
		if err != nil {
			return err
		}
		maybeLockVault(client, v)
		fmt.Printf("Added %d file(s) to vault %q (total %d).\n", added, v.Name, total)
		return nil
	},
}

var vaultFilesCmd = &cobra.Command{
	Use:   "files <vault>",
	Short: "List files in a vault (unlocks it)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		token, err := unlockVault(client, v)
		if err != nil {
			return err
		}
		files, err := client.ListVaultFiles(v.ID, token)
		if err != nil {
			return err
		}
		maybeLockVault(client, v)
		if len(files) == 0 {
			fmt.Println("Vault is empty.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSIZE\tADDED")
		for _, f := range files {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", f.ID, f.Name, humanSize(f.Size), shortDate(f.AddedAt))
		}
		return w.Flush()
	},
}

var vaultRemoveCmd = &cobra.Command{
	Use:   "remove <vault> <file-ids...>",
	Short: "Remove files from a vault (files are not deleted)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		token, err := unlockVault(client, v)
		if err != nil {
			return err
		}
		var ids []int
		for _, s := range args[1:] {
			id, err := strconv.Atoi(s)
			if err != nil {
				return fmt.Errorf("invalid file id %q (must be numeric)", s)
			}
			ids = append(ids, id)
		}
		removed, total, err := client.RemoveVaultFiles(v.ID, token, ids)
		if err != nil {
			return err
		}
		maybeLockVault(client, v)
		fmt.Printf("Removed %d file(s) from vault %q (total %d). Files were moved back to regular storage.\n", removed, v.Name, total)
		return nil
	},
}

var vaultLockCmd = &cobra.Command{
	Use:   "lock <vault>",
	Short: "Lock a vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		if err := client.LockVault(v.ID); err != nil {
			return err
		}
		fmt.Printf("Locked vault %q.\n", v.Name)
		return nil
	},
}

var vaultDeleteCmd = &cobra.Command{
	Use:   "delete <vault>",
	Short: "Delete a vault (its files are moved back to regular storage)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		if !vaultDeleteYes {
			fmt.Printf("Delete vault %q (id %d)? Its %d file(s) will be moved back to regular storage (not deleted). [y/N] ", v.Name, v.ID, v.FilesCount)
			var resp string
			fmt.Scanln(&resp)
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(resp)), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}
		if err := client.DeleteVault(v.ID); err != nil {
			return err
		}
		fmt.Printf("Deleted vault %q. Its files were moved back to regular storage.\n", v.Name)
		return nil
	},
}

var vaultRenameCmd = &cobra.Command{
	Use:   "rename <vault> <new-name>",
	Short: "Rename a vault",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		updated, err := client.UpdateVault(v.ID, args[1], api.VaultOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("Renamed vault to %q.\n", updated.Name)
		return nil
	},
}

var vaultPasswordCmd = &cobra.Command{
	Use:   "password <vault>",
	Short: "Change a vault's password",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		v, err := resolveVault(client, args[0])
		if err != nil {
			return err
		}
		cur, err := vaultCurrentPassword()
		if err != nil {
			return err
		}
		nw, err := promptNewVaultPassword()
		if err != nil {
			return err
		}
		if err := client.ChangeVaultPassword(v.ID, cur, nw); err != nil {
			return err
		}
		fmt.Println("Password changed. The vault has been locked; unlock with the new password.")
		return nil
	},
}

// ---- helpers ----

func resolveVault(client *api.Client, arg string) (api.Vault, error) {
	vaults, err := client.ListVaults()
	if err != nil {
		return api.Vault{}, err
	}
	if id, err := strconv.Atoi(arg); err == nil {
		for _, v := range vaults {
			if v.ID == id {
				return v, nil
			}
		}
		return api.Vault{}, fmt.Errorf("no vault with id %d (run `filejump vault ls`)", id)
	}
	lower := strings.ToLower(strings.TrimSpace(arg))
	var match api.Vault
	count := 0
	for _, v := range vaults {
		if strings.ToLower(v.Name) == lower {
			match = v
			count++
		}
	}
	switch count {
	case 0:
		return api.Vault{}, fmt.Errorf("no vault named %q (run `filejump vault ls`)", arg)
	case 1:
		return match, nil
	default:
		return api.Vault{}, fmt.Errorf("multiple vaults named %q; specify by numeric id", arg)
	}
}

func unlockVault(client *api.Client, v api.Vault) (string, error) {
	pw, err := vaultCurrentPassword()
	if err != nil {
		return "", err
	}
	token, err := client.UnlockVault(v.ID, pw)
	if err != nil {
		return "", err
	}
	return token, nil
}

func maybeLockVault(client *api.Client, v api.Vault) {
	if !vaultKeepUnlocked {
		_ = client.LockVault(v.ID)
	}
}

// vaultCurrentPassword returns the vault password from --password or a prompt.
func vaultCurrentPassword() (string, error) {
	if vaultPassword != "" {
		return vaultPassword, nil
	}
	fmt.Print("Vault password: ")
	p, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Println()
	return p, nil
}

// vaultCreatePassword returns the new vault password for 'create' (with confirm).
func vaultCreatePassword() (string, error) {
	if vaultPassword != "" {
		return vaultPassword, nil
	}
	fmt.Print("New vault password: ")
	p1, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Println()
	fmt.Print("Confirm password: ")
	p2, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Println()
	if p1 != p2 {
		return "", fmt.Errorf("passwords do not match")
	}
	return p1, nil
}

// promptNewVaultPassword returns the new password for 'password' (with confirm).
func promptNewVaultPassword() (string, error) {
	if vaultNewPassword != "" {
		return vaultNewPassword, nil
	}
	fmt.Print("New vault password: ")
	p1, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Println()
	fmt.Print("Confirm password: ")
	p2, err := readPassword()
	if err != nil {
		return "", err
	}
	fmt.Println()
	if p1 != p2 {
		return "", fmt.Errorf("passwords do not match")
	}
	return p1, nil
}

// shortDate trims an ISO timestamp to something compact for tables.
func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func init() {
	vaultCmd.AddCommand(
		vaultLsCmd, vaultShowCmd, vaultCreateCmd, vaultUploadCmd, vaultAddCmd,
		vaultFilesCmd, vaultRemoveCmd, vaultLockCmd, vaultDeleteCmd, vaultRenameCmd,
		vaultPasswordCmd,
	)

	// Shared --password flag on every subcommand that needs to unlock.
	for _, c := range []*cobra.Command{vaultUploadCmd, vaultAddCmd, vaultFilesCmd, vaultRemoveCmd, vaultPasswordCmd} {
		c.Flags().StringVarP(&vaultPassword, "password", "p", "", "vault password (prompted if omitted)")
	}
	for _, c := range []*cobra.Command{vaultUploadCmd, vaultAddCmd, vaultFilesCmd, vaultRemoveCmd} {
		c.Flags().BoolVar(&vaultKeepUnlocked, "keep-unlocked", false, "leave the vault unlocked after the operation")
	}

	vaultCreateCmd.Flags().StringVar(&vaultDescription, "description", "", "vault description")
	vaultCreateCmd.Flags().StringVar(&vaultIcon, "icon", "", "vault icon (emoji or short text)")
	vaultCreateCmd.Flags().BoolVar(&vaultAutoLock, "auto-lock", true, "auto-lock the vault after the timeout")
	vaultCreateCmd.Flags().IntVar(&vaultLockTimeout, "lock-timeout", 30, "auto-lock timeout in minutes (1-1440)")

	vaultPasswordCmd.Flags().StringVar(&vaultNewPassword, "new-password", "", "new vault password (prompted if omitted)")

	vaultDeleteCmd.Flags().BoolVarP(&vaultDeleteYes, "yes", "y", false, "skip the confirmation prompt")

	rootCmd.AddCommand(vaultCmd)
}
