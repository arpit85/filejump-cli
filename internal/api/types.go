package api

import "encoding/json"

// Envelope is the standard { success, data, message, errors } API response.
type Envelope struct {
	Success      bool            `json:"success"`
	Data         json.RawMessage `json:"data,omitempty"`
	Message      string          `json:"message,omitempty"`
	Errors       map[string]any  `json:"errors,omitempty"`
	Pagination   *Pagination     `json:"pagination,omitempty"`
	TwoFactorReq bool            `json:"two_factor_required,omitempty"`
}

type Pagination struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

// File is a file listing item (subset of fields).
type File struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	Path         string `json:"path"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// Folder is a folder listing item.
type Folder struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	ParentID *int   `json:"parent_id"`
}

// Workspace is one entry from GET /api/workspaces.
type Workspace struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	OwnerID int    `json:"owner_id"`
	MyRole  string `json:"my_role"`
}

// Share is the data payload returned by the file share endpoints.
type Share struct {
	URL              string `json:"url"`
	Token            string `json:"token"`
	ExpiresAt        string `json:"expires_at"`
	RequiresPassword bool   `json:"requires_password"`
	IsActive         bool   `json:"is_active"`
	MaxDownloads     *int   `json:"max_downloads"`
	DownloadCount    int    `json:"download_count"`
	AllowDownload    bool   `json:"allow_download"`
}

// ShareOptions are the optional parameters for creating/updating a share.
type ShareOptions struct {
	Password      string
	ExpiresAt     string // ISO 8601 datetime, or "" for no expiry
	MaxDownloads  int    // 0 = leave unset (no limit)
	Active        *bool
	AllowDownload *bool
}

// LoginResponse is the data payload returned by POST /api/auth/login.
type LoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	User      struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
}

// UploadURLResponse is the raw (un-wrapped) response from POST /api/files/upload-url.
type UploadURLResponse struct {
	UploadURL       string            `json:"upload_url"`
	Headers         map[string]string `json:"headers"`
	Path            string            `json:"path"`
	StorageServerID int               `json:"storage_server_id"`
	Method          string            `json:"method"`
	UploadServer    string            `json:"upload_server"`
}

// CompleteUploadRequest is the body for POST /api/files/upload-complete.
type CompleteUploadRequest struct {
	Path            string
	StorageServerID int
	Name            string
	Size            int64
	MimeType        string
	FolderID        *int
}

// errorResponse matches the { error, code } shape used by the upload endpoints.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// SyncEntry is one entry in a /api/sync/delta response.
type SyncEntry struct {
	Tag             string `json:".tag"`
	ID              int    `json:"id"`
	Name            string `json:"name"`
	PathLower       string `json:"path_lower"`
	PathDisplay     string `json:"path_display"`
	ParentID        *int   `json:"parent_id"`
	FolderID        *int   `json:"folder_id"`
	Size            int64  `json:"size"`
	Hash            string `json:"hash"`
	ContentHash     string `json:"content_hash"`
	Status          string `json:"status"`
	IsDeleted       bool   `json:"is_deleted"`
	DeletedAt       string `json:"deleted_at"`
	ServerModified  string `json:"server_modified"`
	DownloadURL     string `json:"download_url"`
}

// SyncDeltaResponse is the response from POST /api/sync/delta.
type SyncDeltaResponse struct {
	Success bool         `json:"success"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"has_more"`
	Entries []SyncEntry  `json:"entries"`
}
