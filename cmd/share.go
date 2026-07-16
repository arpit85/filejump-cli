package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var (
	sharePassword   string
	shareExpires    string
	shareMaxDL      int
	shareViewOnly   bool
	shareNoExpiry   bool
	shareInactive   bool
	shareAllowDL    bool
	shareDisallowDL bool
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Create and manage public share links for files",
	Long: `Create a public share link for a file and print its hotlinkable URLs.

The fastest form is:

    filejump share /Photos/cat.jpg

This mints (or updates) a share link and prints three URLs:
  - page:     the share landing page (https://.../s/<token>)
  - content:  inline stream URL — use this to hotlink in <img>/<video>/datasets
  - download: attachment URL (counts against max_downloads)

Subcommands:
  filejump share show <path>     show the existing share link for a file
  filejump share revoke <path>   delete a file's share link

Note: the share API currently manages personal-space files only. If a workspace
is active, switch to personal with "-w 0" or "filejump workspace reset".`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runShareCreate(cmd, args[0])
	},
}

var shareShowCmd = &cobra.Command{
	Use:   "show <path>",
	Short: "Show the existing share link for a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		file, err := resolveAnyFile(client, args[0])
		if err != nil {
			return err
		}
		s, err := client.GetShare(file.ID)
		if err != nil {
			return err
		}
		if s == nil {
			fmt.Println("No share link for this file.")
			return nil
		}
		printShare(s, client.ServerRoot)
		return nil
	},
}

var shareRevokeCmd = &cobra.Command{
	Use:   "revoke <path>",
	Short: "Delete a file's share link",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		file, err := resolveAnyFile(client, args[0])
		if err != nil {
			return err
		}
		if err := client.DeleteShare(file.ID); err != nil {
			return err
		}
		fmt.Println("Share link revoked.")
		return nil
	},
}

func runShareCreate(cmd *cobra.Command, remotePath string) error {
	client, _ := requireClient()
	file, err := resolveAnyFile(client, remotePath)
	if err != nil {
		return err
	}

	opts := api.ShareOptions{Password: sharePassword}
	if !shareNoExpiry && shareExpires != "" {
		exp, err := parseExpiry(shareExpires)
		if err != nil {
			return err
		}
		opts.ExpiresAt = exp
	}
	if shareMaxDL > 0 {
		opts.MaxDownloads = shareMaxDL
	}
	if cmd.Flags().Changed("inactive") {
		v := !shareInactive
		opts.Active = &v
	}
	if cmd.Flags().Changed("view-only") || cmd.Flags().Changed("allow-download") || cmd.Flags().Changed("disallow-download") {
		allow := !shareViewOnly && !shareDisallowDL
		opts.AllowDownload = &allow
	}

	s, err := client.CreateShare(file.ID, opts)
	if err != nil {
		return err
	}
	printShare(s, client.ServerRoot)
	return nil
}

// resolveAnyFile resolves a remote file path, preferring the active workspace
// context (so workspace files can be located), even though the share endpoints
// themselves only operate on personal-space files.
func resolveAnyFile(client *api.Client, p string) (*api.File, error) {
	resolver := pathresolve.New(client)
	return resolver.ResolveFile(p)
}

// printShare prints the page/content/download URLs and key settings.
func printShare(s *api.Share, serverRoot string) {
	pageURL := s.URL
	if pageURL == "" && s.Token != "" {
		pageURL = serverRoot + "/s/" + s.Token
	}
	contentURL := ""
	downloadURL := ""
	if s.Token != "" {
		contentURL = serverRoot + "/s/" + s.Token + "/content"
		downloadURL = serverRoot + "/s/" + s.Token + "/download"
	}
	fmt.Println("Share link created:")
	if pageURL != "" {
		fmt.Printf("  page:     %s\n", pageURL)
	}
	if contentURL != "" {
		fmt.Printf("  content:   %s   (hotlink: <img src>, <video src>, datasets)\n", contentURL)
	}
	if downloadURL != "" {
		fmt.Printf("  download:  %s\n", downloadURL)
	}
	fmt.Println("Settings:")
	fmt.Printf("  active:            %v\n", s.IsActive)
	fmt.Printf("  allow_download:    %v\n", s.AllowDownload)
	fmt.Printf("  requires_password: %v\n", s.RequiresPassword)
	if s.ExpiresAt != "" {
		fmt.Printf("  expires_at:        %s\n", s.ExpiresAt)
	} else {
		fmt.Println("  expires_at:        (never)")
	}
	if s.MaxDownloads != nil {
		fmt.Printf("  max_downloads:     %d (used %d)\n", *s.MaxDownloads, s.DownloadCount)
	} else {
		fmt.Println("  max_downloads:     (unlimited)")
	}
}

// parseExpiry accepts a duration (e.g. "7d", "24h", "30m") or an ISO 8601
// datetime, returning the string the API expects.
func parseExpiry(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// If it looks like an absolute datetime (contains 'T' or '-' date separators
	// and isn't a pure duration), pass it through.
	if strings.ContainsAny(s, "T:/") && !looksLikeDuration(s) {
		return s, nil
	}
	dur, err := parseDuration(s)
	if err != nil {
		return "", fmt.Errorf("--expires: %v (try e.g. 7d, 24h, 30m, or an ISO datetime)", err)
	}
	return time.Now().Add(dur).UTC().Format(time.RFC3339), nil
}

func looksLikeDuration(s string) bool {
	if s == "" {
		return false
	}
	// Pure duration tokens like 7d, 12h, 30m, 2h30m.
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == 'd' || r == 'h' || r == 'm' || r == 's':
		default:
			return false
		}
	}
	return true
}

// parseDuration extends time.ParseDuration with day ("d") support.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func init() {
	shareCmd.AddCommand(shareShowCmd, shareRevokeCmd)

	shareCmd.Flags().StringVar(&sharePassword, "password", "", "protect the share with a password")
	shareCmd.Flags().StringVar(&shareExpires, "expires", "", "expiry as a duration (e.g. 7d, 24h, 30m) or ISO datetime; omit for never")
	shareCmd.Flags().IntVar(&shareMaxDL, "max-downloads", 0, "cap total downloads (0 = unlimited)")
	shareCmd.Flags().BoolVar(&shareViewOnly, "view-only", false, "disable downloads (content still streamable)")
	shareCmd.Flags().BoolVar(&shareInactive, "inactive", false, "create the share in an inactive state")
	shareCmd.Flags().BoolVar(&shareNoExpiry, "no-expiry", false, "explicitly never expire (default when --expires omitted)")
	shareCmd.Flags().BoolVar(&shareAllowDL, "allow-download", false, "explicitly allow downloads")
	shareCmd.Flags().BoolVar(&shareDisallowDL, "disallow-download", false, "explicitly disallow downloads")
	_ = shareAllowDL
	_ = shareDisallowDL

	rootCmd.AddCommand(shareCmd)
}
