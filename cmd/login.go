package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/arpit85/filejump-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginServer string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and store an API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginServer == "" {
			fmt.Print("Server URL (e.g. https://filejump.com): ")
			fmt.Scanln(&loginServer)
		}
		loginServer = strings.TrimRight(strings.TrimSpace(loginServer), "/")
		if loginServer == "" {
			return fmt.Errorf("server URL is required")
		}

		var email string
		fmt.Print("Email: ")
		fmt.Scanln(&email)

		fmt.Print("Password: ")
		pwd, err := readPassword()
		if err != nil {
			return err
		}
		fmt.Println()

		client := api.New(loginServer, "")
		lr, err := client.Login(email, pwd, "")
		if err == api.ErrTwoFactorRequired {
			fmt.Print("Two-factor code: ")
			var code string
			fmt.Scanln(&code)
			lr, err = client.Login(email, pwd, strings.TrimSpace(code))
		}
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		cfg := &config.Config{
			Server: loginServer,
			Token:  lr.Token,
			Email:  lr.User.Email,
		}
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Logged in as %s (%s)\n", lr.User.Name, lr.User.Email)
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored API token and clear local config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token != "" && cfg.Server != "" {
			_ = api.New(cfg.Server, cfg.Token).Logout()
		}
		if err := config.Delete(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently logged in account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		if cfg.Token == "" {
			fmt.Println("Not logged in.")
			return nil
		}
		fmt.Printf("Server: %s\n", cfg.Server)
		if cfg.Email != "" {
			fmt.Printf("Email:  %s\n", cfg.Email)
		}
		if cfg.WorkspaceID == 0 {
			fmt.Println("Workspace: personal space")
		} else if cfg.WorkspaceName != "" {
			fmt.Printf("Workspace: %s (id %d)\n", cfg.WorkspaceName, cfg.WorkspaceID)
		} else {
			fmt.Printf("Workspace: id %d\n", cfg.WorkspaceID)
		}
		return nil
	},
}

func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		bytes, err := term.ReadPassword(fd)
		return string(bytes), err
	}
	// Non-interactive (piped): read a line.
	var s string
	fmt.Scanln(&s)
	return s, nil
}

func init() {
	loginCmd.Flags().StringVar(&loginServer, "server", "", "FileJump server URL")
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
}
