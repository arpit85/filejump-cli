package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arpit85/filejump-cli/internal/api"
)

// normalizeRoot ensures a remote root like "/" or "/Photos" (leading slash, no trailing slash except root).
func normalizeRoot(r string) string {
	if r == "" {
		return "/"
	}
	if !strings.HasPrefix(r, "/") {
		r = "/" + r
	}
	r = filepath.Clean(r)
	return r
}

// relFromRoot returns the path below remoteRoot for a path_display, or ok=false if not under it.
// The root itself maps to "" (the local directory root), not a subfolder.
func relFromRoot(pathDisplay, remoteRoot string) (string, bool) {
	if pathDisplay == remoteRoot {
		return "", true
	}
	if remoteRoot == "/" {
		return strings.TrimPrefix(pathDisplay, "/"), true
	}
	prefix := strings.TrimSuffix(remoteRoot, "/") + "/"
	if !strings.HasPrefix(pathDisplay+"/", prefix) {
		return "", false
	}
	return strings.TrimPrefix(pathDisplay, prefix), true
}

// localDest joins localDir with a slash-relative remote path, converting to OS separators.
func localDest(localDir, rel string) string {
	return filepath.Join(localDir, filepath.FromSlash(rel))
}

// Pull applies all remote changes since the stored cursor to the local directory.
func Pull(client *api.Client, state *State, localDir string) (int, error) {
	remoteRoot := normalizeRoot(state.RemoteRoot)
	state.RemoteRoot = remoteRoot
	ops := 0
	cursor := state.Cursor
	for {
		resp, err := client.SyncDelta(cursor, 500)
		if err != nil {
			return ops, err
		}
		for i := range resp.Entries {
			applied, err := applyPullEntry(client, state, localDir, remoteRoot, resp.Entries[i])
			if err != nil {
				return ops, err
			}
			if applied {
				ops++
			}
		}
		cursor = resp.Cursor
		if !resp.HasMore {
			break
		}
	}
	state.Cursor = cursor
	return ops, nil
}

func applyPullEntry(client *api.Client, state *State, localDir, remoteRoot string, e api.SyncEntry) (bool, error) {
	if e.PathDisplay == "" {
		return false, nil
	}
	rel, ok := relFromRoot(e.PathDisplay, remoteRoot)
	if !ok {
		return false, nil
	}

	if e.Tag == "folder" {
		dest := localDest(localDir, rel)
		if e.IsDeleted || e.Status == "deleted" {
			_ = os.Remove(dest) // best-effort rmdir
			return true, nil
		}
		if err := os.MkdirAll(dest, 0755); err != nil {
			return false, err
		}
		return true, nil
	}

	// File entry.
	dest := localDest(localDir, rel)
	known, knownOK := state.Files[e.PathDisplay]

	if e.IsDeleted || e.Status == "deleted" {
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		delete(state.Files, e.PathDisplay)
		return true, nil
	}

	// Unchanged remote file already present locally: just refresh metadata.
	// Treat as up-to-date if hashes match, or if the local hash is unknown
	// (e.g. just pushed) but the local size matches the remote size.
	if knownOK && fileExists(dest) {
		if known.RemoteHash == e.ContentHash && known.RemoteHash != "" {
			state.Files[e.PathDisplay] = refreshLocal(known, e, dest)
			return false, nil
		}
		if known.RemoteHash == "" && known.LocalSize == e.Size {
			state.Files[e.PathDisplay] = refreshLocal(known, e, dest)
			return false, nil
		}
	}

	// Conflict: local file changed since last sync and remote also changed.
	if knownOK && fileExists(dest) {
		if localChanged(dest, known) && remoteChanged(known, e) {
			conflict := conflictPath(dest)
			if err := os.Rename(dest, conflict); err != nil {
				return false, fmt.Errorf("conflict rename %s: %w", dest, err)
			}
			fmt.Fprintf(os.Stderr, "conflict: kept local copy as %s; downloading remote %s\n", conflict, dest)
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return false, err
	}
	if e.DownloadURL == "" {
		return false, fmt.Errorf("no download_url for %s", e.PathDisplay)
	}
	if err := client.DownloadURL(e.DownloadURL, dest); err != nil {
		return false, fmt.Errorf("download %s: %w", e.PathDisplay, err)
	}
	state.Files[e.PathDisplay] = refreshLocal(FileEntry{
		RemoteID:       e.ID,
		RemoteHash:      e.ContentHash,
		RemoteModified:  e.ServerModified,
	}, e, dest)
	return true, nil
}

// refreshLocal fills in LocalMTime/LocalSize from the file on disk.
func refreshLocal(entry FileEntry, e api.SyncEntry, dest string) FileEntry {
	entry.RemoteID = e.ID
	entry.RemoteHash = e.ContentHash
	entry.RemoteModified = e.ServerModified
	if fi, err := os.Stat(dest); err == nil {
		entry.LocalMTime = fi.ModTime().Unix()
		entry.LocalSize = fi.Size()
	}
	return entry
}

func localChanged(dest string, known FileEntry) bool {
	fi, err := os.Stat(dest)
	if err != nil {
		return true
	}
	return fi.ModTime().Unix() != known.LocalMTime || fi.Size() != known.LocalSize
}

func remoteChanged(known FileEntry, e api.SyncEntry) bool {
	return e.ContentHash != "" && known.RemoteHash != "" && e.ContentHash != known.RemoteHash
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// conflictPath returns dest with a ".conflict" marker before the extension.
func conflictPath(dest string) string {
	ext := filepath.Ext(dest)
	base := strings.TrimSuffix(dest, ext)
	return base + ".conflict" + ext
}
