package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arpit85/filejump-cli/internal/api"
	"github.com/arpit85/filejump-cli/internal/pathresolve"
	"github.com/spf13/cobra"
)

var uploadMkdir bool

var uploadCmd = &cobra.Command{
	Use:   "upload <local-path> [remote-folder]",
	Short: "Upload a local file (or directory tree) to a remote folder",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _ := requireClient()
		local := args[0]
		remoteDir := ""
		if len(args) > 1 {
			remoteDir = args[1]
		}

		info, err := os.Stat(local)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return uploadDir(client, local, remoteDir)
		}
		return uploadOne(client, local, remoteDir)
	},
}

func uploadOne(client *api.Client, local, remoteDir string) error {
	resolver := pathresolve.New(client)
	var folderID *int
	if remoteDir != "" {
		var err error
		folderID, err = resolver.ResolveFolder(remoteDir)
		if err != nil {
			if uploadMkdir {
				folderID, err = resolver.EnsureFolder(remoteDir)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	fmt.Printf("Uploading %s ... ", filepath.Base(local))
	file, err := uploadBest(client, local, folderID)
	if err != nil {
		fmt.Println("failed")
		return err
	}
	fmt.Printf("done (%s)\n", humanSize(file.Size))
	return nil
}

// presignedThreshold: files at/above this size use the presigned S3 flow.
const presignedThreshold int64 = 50 * 1024 * 1024

// uploadBest picks presigned S3 upload for large files (falling back to
// multipart proxy upload when the server reports no S3 support) and uses the
// simple multipart endpoint for small files.
func uploadBest(client *api.Client, local string, folderID *int) (*api.File, error) {
	info, err := os.Stat(local)
	if err != nil {
		return nil, err
	}
	size := info.Size()

	if size >= presignedThreshold {
		file, err := uploadPresigned(client, local, folderID, size)
		if err == nil {
			return file, nil
		}
		if err == api.ErrFallbackToProxy {
			// Fall through to multipart proxy upload.
		} else {
			return nil, err
		}
	}

	return client.Upload(local, folderID)
}

// uploadPresigned performs the three-step presigned S3 upload.
func uploadPresigned(client *api.Client, local string, folderID *int, size int64) (*api.File, error) {
	name := filepath.Base(local)
	uu, err := client.GetUploadURL(name, size, folderID)
	if err != nil {
		return nil, err
	}
	if err := client.PutPresigned(uu.UploadURL, uu.Headers, local, size); err != nil {
		return nil, err
	}
	return client.CompleteUpload(api.CompleteUploadRequest{
		Path:            uu.Path,
		StorageServerID: uu.StorageServerID,
		Name:            name,
		Size:            size,
		MimeType:        detectMime(name),
		FolderID:        folderID,
	})
}

func uploadDir(client *api.Client, local, remoteDir string) error {
	return filepath.Walk(local, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(local, p)
		relDir := filepath.ToSlash(filepath.Dir(rel))
		target := remoteDir
		if relDir != "." {
			target = target + "/" + relDir
		}
		return uploadOne(client, p, target)
	})
}

func init() {
	uploadCmd.Flags().BoolVarP(&uploadMkdir, "mkdir", "p", false, "create remote folder if missing")
	rootCmd.AddCommand(uploadCmd)
}
