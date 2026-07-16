package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client is an authenticated FileJump API client.
type Client struct {
	ServerRoot string // e.g. https://filejump.com
	BaseURL    string // e.g. https://filejump.com/api
	Token      string // Sanctum bearer token
	HTTP       *http.Client
}

// New constructs a client from a server root (e.g. https://filejump.com).
func New(server, token string) *Client {
	server = strings.TrimRight(server, "/")
	return &Client{
		ServerRoot: server,
		BaseURL:    server + "/api",
		Token:      token,
		HTTP:       &http.Client{Timeout: 30 * time.Minute},
	}
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status  int
	Message string
	Errors  map[string]any
}

func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("API %d: %s %v", e.Status, e.Message, e.Errors)
	}
	return fmt.Sprintf("API %d: %s", e.Status, e.Message)
}

// IsUnauthorized reports whether the error is a 401.
func IsUnauthorized(err error) bool {
	if ae, ok := err.(*APIError); ok {
		return ae.Status == http.StatusUnauthorized
	}
	return false
}

// do performs a request and decodes the envelope. Caller passes the body reader.
func (c *Client) do(method, path string, body io.Reader, headers map[string]string) (*Envelope, []byte, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// Non-JSON response (e.g. streamed download handled separately).
		if resp.StatusCode >= 400 {
			return nil, raw, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
		}
		return nil, raw, nil
	}

	if resp.StatusCode >= 400 {
		return &env, raw, &APIError{Status: resp.StatusCode, Message: env.Message, Errors: env.Errors}
	}
	return &env, raw, nil
}

// Login posts credentials and returns the bearer token.
func (c *Client) Login(email, password, twoFactorCode string) (*LoginResponse, error) {
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	if twoFactorCode != "" {
		form.Set("two_factor_code", twoFactorCode)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("login: unexpected response: %s", strings.TrimSpace(string(raw)))
	}
	if env.TwoFactorReq {
		return nil, ErrTwoFactorRequired
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Message: env.Message, Errors: env.Errors}
	}

	var lr LoginResponse
	if err := json.Unmarshal(env.Data, &lr); err != nil {
		return nil, fmt.Errorf("login: decode: %w", err)
	}
	return &lr, nil
}

// ErrTwoFactorRequired signals that a 2FA code is needed.
var ErrTwoFactorRequired = fmt.Errorf("two-factor authentication required")

// Logout deletes the current token server-side.
func (c *Client) Logout() error {
	_, _, err := c.do(http.MethodPost, "/auth/logout", nil, nil)
	return err
}

// ListFolders returns folders under the given parent (nil = root).
func (c *Client) ListFolders(parentID *int) ([]Folder, error) {
	q := url.Values{}
	if parentID != nil {
		q.Set("parent_id", strconv.Itoa(*parentID))
	}
	q.Set("per_page", "100")
	var all []Folder
	page := 1
	for {
		q.Set("page", strconv.Itoa(page))
		env, _, err := c.do(http.MethodGet, "/folders?"+q.Encode(), nil, nil)
		if err != nil {
			return nil, err
		}
		var batch []Folder
		if err := json.Unmarshal(env.Data, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if env.Pagination == nil || env.Pagination.CurrentPage >= env.Pagination.LastPage {
			break
		}
		page++
	}
	return all, nil
}

// ListFiles returns files in the given folder (nil = root).
func (c *Client) ListFiles(folderID *int) ([]File, error) {
	q := url.Values{}
	if folderID != nil {
		q.Set("folder_id", strconv.Itoa(*folderID))
	}
	q.Set("per_page", "100")
	var all []File
	page := 1
	for {
		q.Set("page", strconv.Itoa(page))
		env, _, err := c.do(http.MethodGet, "/files?"+q.Encode(), nil, nil)
		if err != nil {
			return nil, err
		}
		var batch []File
		if err := json.Unmarshal(env.Data, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if env.Pagination == nil || env.Pagination.CurrentPage >= env.Pagination.LastPage {
			break
		}
		page++
	}
	return all, nil
}

// CreateFolder creates a folder with the given name under parentID (nil = root).
func (c *Client) CreateFolder(name string, parentID *int) (*Folder, error) {
	form := url.Values{}
	form.Set("name", name)
	if parentID != nil {
		form.Set("parent_id", strconv.Itoa(*parentID))
	}
	env, _, err := c.do(http.MethodPost, "/folders", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		return nil, err
	}
	var f Folder
	if err := json.Unmarshal(env.Data, &f); err != nil {
		// Some responses wrap the folder; tolerate either.
		return nil, nil
	}
	return &f, nil
}

// DeleteFile removes a file by ID.
func (c *Client) DeleteFile(id int) error {
	_, _, err := c.do(http.MethodDelete, "/files/"+strconv.Itoa(id), nil, nil)
	return err
}

// MoveFile moves a single file to a new folder (destFolderID). It does NOT rename.
func (c *Client) MoveFile(id int, destFolderID *int) error {
	form := url.Values{}
	if destFolderID != nil {
		form.Set("folder_id", strconv.Itoa(*destFolderID))
	}
	_, _, err := c.do(http.MethodPut, "/files/"+strconv.Itoa(id)+"/move", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	return err
}

// RenameFile renames a file in place (PUT /api/files/{id}).
func (c *Client) RenameFile(id int, newName string) error {
	form := url.Values{}
	form.Set("name", newName)
	_, _, err := c.do(http.MethodPut, "/files/"+strconv.Itoa(id), strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	return err
}

// Upload uploads a local file into folderID (nil = root) via multipart.
func (c *Client) Upload(localPath string, folderID *int) (*File, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if folderID != nil {
		_ = w.WriteField("folder_id", strconv.Itoa(*folderID))
	}
	part, err := w.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	env, _, err := c.do(http.MethodPost, "/files/upload", &buf, map[string]string{
		"Content-Type": w.FormDataContentType(),
	})
	if err != nil {
		return nil, err
	}
	var file File
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &file)
	}
	return &file, nil
}

// Download streams a file by ID to destPath.
func (c *Client) Download(id int, destPath string) error {
	return c.DownloadURL(c.BaseURL+"/files/download/"+strconv.Itoa(id), destPath)
}

// DownloadURL streams a file from an absolute or server-relative URL to destPath.
func (c *Client) DownloadURL(rawURL, destPath string) error {
	u := rawURL
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		if strings.HasPrefix(u, "/") {
			u = c.ServerRoot + u
		} else {
			u = c.ServerRoot + "/" + u
		}
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

// ErrFallbackToProxy signals that the server has no S3 presigned-upload support
// and the caller should fall back to multipart proxy upload.
var ErrFallbackToProxy = fmt.Errorf("direct presigned upload not available")

// GetUploadURL requests a presigned S3 PUT URL for a file.
func (c *Client) GetUploadURL(name string, size int64, folderID *int) (*UploadURLResponse, error) {
	form := url.Values{}
	form.Set("name", name)
	form.Set("size", strconv.FormatInt(size, 10))
	if folderID != nil {
		form.Set("folder_id", strconv.Itoa(*folderID))
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/files/upload-url", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict {
		var er errorResponse
		_ = json.Unmarshal(raw, &er)
		if er.Code == "FALLBACK_TO_PROXY" {
			return nil, ErrFallbackToProxy
		}
		return nil, &APIError{Status: resp.StatusCode, Message: er.Error}
	}
	if resp.StatusCode >= 400 {
		var er errorResponse
		_ = json.Unmarshal(raw, &er)
		return nil, &APIError{Status: resp.StatusCode, Message: er.Error}
	}

	var u UploadURLResponse
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, fmt.Errorf("upload-url: decode: %w", err)
	}
	return &u, nil
}

// PutPresigned streams a local file to a presigned S3 URL with the given headers.
func (c *Client) PutPresigned(uploadURL string, headers map[string]string, localPath string, size int64) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return err
	}
	req.ContentLength = size
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 PUT failed: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// CompleteUpload registers a presigned-uploaded file in the database.
func (c *Client) CompleteUpload(r CompleteUploadRequest) (*File, error) {
	form := url.Values{}
	form.Set("path", r.Path)
	form.Set("storage_server_id", strconv.Itoa(r.StorageServerID))
	form.Set("name", r.Name)
	form.Set("size", strconv.FormatInt(r.Size, 10))
	form.Set("mime_type", r.MimeType)
	if r.FolderID != nil {
		form.Set("folder_id", strconv.Itoa(*r.FolderID))
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/files/upload-complete", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var er errorResponse
		_ = json.Unmarshal(raw, &er)
		return nil, &APIError{Status: resp.StatusCode, Message: er.Error}
	}

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("upload-complete: decode: %w", err)
	}
	var file File
	_ = json.Unmarshal(env.Data, &file)
	return &file, nil
}

// SyncDelta fetches incremental changes since the cursor (empty = all history).
func (c *Client) SyncDelta(cursor string, limit int) (*SyncDeltaResponse, error) {
	form := url.Values{}
	if cursor != "" {
		form.Set("cursor", cursor)
	}
	if limit > 0 {
		form.Set("limit", strconv.Itoa(limit))
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/sync/delta", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var env Envelope
		_ = json.Unmarshal(raw, &env)
		return nil, &APIError{Status: resp.StatusCode, Message: env.Message}
	}

	var s SyncDeltaResponse
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("sync/delta: decode: %w", err)
	}
	return &s, nil
}
