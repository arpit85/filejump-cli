package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// FileEntry tracks one synced file's local + remote metadata.
type FileEntry struct {
	RemoteID       int    `json:"remote_id"`
	RemoteHash     string `json:"remote_hash"`
	RemoteModified string `json:"remote_modified"`
	LocalMTime      int64  `json:"local_mtime"` // unix seconds
	LocalSize      int64  `json:"local_size"`
}

// State is the persisted sync state for one local directory <-> remote root pair.
type State struct {
	LocalPath  string                `json:"local_path"`
	RemoteRoot string                `json:"remote_root"`
	Cursor     string                `json:"cursor"`
	Files      map[string]FileEntry  `json:"files"` // key = remote path_display (e.g. /Photos/x.jpg)
}

// statePath returns the on-disk path for a given local directory's state file.
func statePath(localDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(localDir)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(filepath.Clean(abs)))
	name := hex.EncodeToString(h[:]) + ".json"
	return filepath.Join(home, ".config", "filejump", "sync", name), nil
}

// LoadState reads the sync state for localDir. Returns a fresh State if absent.
func LoadState(localDir, remoteRoot string) (*State, error) {
	path, err := statePath(localDir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				LocalPath:  localDir,
				RemoteRoot: remoteRoot,
				Files:      map[string]FileEntry{},
			}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse sync state: %w", err)
	}
	if s.Files == nil {
		s.Files = map[string]FileEntry{}
	}
	s.LocalPath = localDir
	s.RemoteRoot = remoteRoot
	return &s, nil
}

// Save writes the state to disk.
func (s *State) Save() error {
	path, err := statePath(s.LocalPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// SortedPaths returns the file keys in deterministic order.
func (s *State) SortedPaths() []string {
	keys := make([]string, 0, len(s.Files))
	for k := range s.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
