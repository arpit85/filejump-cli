package pathresolve

import (
	"fmt"
	"path"
	"strings"

	"github.com/arpit85/filejump-cli/internal/api"
)

// Resolver walks the folder tree to map human paths to folder IDs.
type Resolver struct {
	client *api.Client
	cache  map[int][]api.Folder // parentID -> children
}

func New(client *api.Client) *Resolver {
	return &Resolver{client: client, cache: map[int][]api.Folder{}}
}

// folders returns children of a parent (nil = root), using a cache.
func (r *Resolver) folders(parentID *int) ([]api.Folder, error) {
	key := 0
	if parentID != nil {
		key = *parentID
	}
	if cached, ok := r.cache[key]; ok {
		return cached, nil
	}
	folders, err := r.client.ListFolders(parentID)
	if err != nil {
		return nil, err
	}
	r.cache[key] = folders
	return folders, nil
}

// ResolveFolder walks /a/b/c and returns the folder ID. Returns nil, nil for root.
func (r *Resolver) ResolveFolder(p string) (*int, error) {
	p = cleanPath(p)
	if p == "" || p == "/" {
		return nil, nil
	}
	segments := strings.Split(strings.Trim(p, "/"), "/")
	var parentID *int
	for _, seg := range segments {
		children, err := r.folders(parentID)
		if err != nil {
			return nil, err
		}
		var found *api.Folder
		for i := range children {
			if children[i].Name == seg {
				found = &children[i]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("folder not found: %s", seg)
		}
		id := found.ID
		parentID = &id
	}
	return parentID, nil
}

// ResolveFile resolves /a/b/file.jpg to a File. The last segment is a file name.
func (r *Resolver) ResolveFile(p string) (*api.File, error) {
	p = cleanPath(p)
	if p == "" {
		return nil, fmt.Errorf("empty path")
	}
	dir, name := path.Split(p)
	folderID, err := r.ResolveFolder(dir)
	if err != nil {
		return nil, err
	}
	files, err := r.client.ListFiles(folderID)
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].Name == name {
			return &files[i], nil
		}
	}
	return nil, fmt.Errorf("file not found: %s", name)
}

// EnsureFolder creates /a/b/c if missing and returns its ID.
func (r *Resolver) EnsureFolder(p string) (*int, error) {
	p = cleanPath(p)
	if p == "" || p == "/" {
		return nil, nil
	}
	segments := strings.Split(strings.Trim(p, "/"), "/")
	var parentID *int
	for _, seg := range segments {
		children, err := r.folders(parentID)
		if err != nil {
			return nil, err
		}
		var found *api.Folder
		for i := range children {
			if children[i].Name == seg {
				found = &children[i]
				break
			}
		}
		if found != nil {
			id := found.ID
			parentID = &id
			continue
		}
		created, err := r.client.CreateFolder(seg, parentID)
		if err != nil {
			return nil, err
		}
		if created != nil {
			r.cache[parentKey(parentID)] = append(r.cache[parentKey(parentID)], *created)
			id := created.ID
			parentID = &id
		}
	}
	return parentID, nil
}

func parentKey(parentID *int) int {
	if parentID == nil {
		return 0
	}
	return *parentID
}

func cleanPath(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}
