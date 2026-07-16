package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arpit85/filejump-cli/internal/api"
)

// UploadFn uploads a local file into a remote folder path (e.g. "/Photos/2026")
// and returns the created file record. Provided by the cmd package to avoid a
// circular import.
type UploadFn func(localPath, remoteFolder string) (*api.File, error)

// Push walks the local directory and uploads new/modified files, and deletes
// remote files that were removed locally. Returns the number of operations.
func Push(client *api.Client, state *State, localDir string, upload UploadFn) (int, error) {
	remoteRoot := normalizeRoot(state.RemoteRoot)
	state.RemoteRoot = remoteRoot
	ops := 0
	seen := map[string]bool{}

	walkErr := filepath.Walk(localDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, ".") { // skip dotfiles (.DS_Store, .gitkeep, ...)
			return nil
		}
		rel, err := filepath.Rel(localDir, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		remotePath := joinRemote(remoteRoot, relSlash)
		seen[remotePath] = true

		known, knownOK := state.Files[remotePath]
		if knownOK && !localChanged(p, known) {
			return nil // unchanged
		}

		remoteFolder := remoteDir(remotePath)
		fmt.Printf("Pushing %s ... ", relSlash)
		file, err := upload(p, remoteFolder)
		if err != nil {
			fmt.Println("failed")
			return fmt.Errorf("upload %s: %w", relSlash, err)
		}
		entry := FileEntry{
			RemoteID:      file.ID,
			RemoteModified: file.UpdatedAt,
		}
		if fi, err := os.Stat(p); err == nil {
			entry.LocalMTime = fi.ModTime().Unix()
			entry.LocalSize = fi.Size()
		}
		state.Files[remotePath] = entry
		ops++
		fmt.Println("done")
		return nil
	})
	if walkErr != nil {
		return ops, walkErr
	}

	// Delete remote files that no longer exist locally.
	for _, key := range state.SortedPaths() {
		if seen[key] {
			continue
		}
		entry := state.Files[key]
		if entry.RemoteID == 0 {
			delete(state.Files, key)
			continue
		}
		fmt.Printf("Deleting remote %s ... ", key)
		if err := client.DeleteFile(entry.RemoteID); err != nil {
			fmt.Println("failed")
			return ops, fmt.Errorf("delete %s: %w", key, err)
		}
		delete(state.Files, key)
		ops++
		fmt.Println("done")
	}
	return ops, nil
}

// joinRemote combines a remote root and a slash-relative path into a full path.
func joinRemote(root, rel string) string {
	if root == "/" {
		return "/" + rel
	}
	return strings.TrimSuffix(root, "/") + "/" + rel
}

// remoteDir returns the folder portion of a full remote path.
func remoteDir(remotePath string) string {
	idx := strings.LastIndex(remotePath, "/")
	if idx <= 0 {
		return "/"
	}
	return remotePath[:idx]
}
