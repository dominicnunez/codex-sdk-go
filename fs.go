package codex

import (
	"context"
	"encoding/json"
)

// FsReadFileParams reads a file from the host filesystem.
type FsReadFileParams struct {
	Path string `json:"path"`
}

// FsReadFileResponse contains base64-encoded file contents.
type FsReadFileResponse struct {
	DataBase64 string `json:"dataBase64"`
}

func (r *FsReadFileResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "dataBase64"); err != nil {
		return err
	}
	type wire FsReadFileResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = FsReadFileResponse(decoded)
	return nil
}

// FsWriteFileParams writes a file on the host filesystem.
type FsWriteFileParams struct {
	DataBase64 string `json:"dataBase64"`
	Path       string `json:"path"`
}

// FsWriteFileResponse is the empty response from fs/writeFile.
type FsWriteFileResponse struct{}

// FsCreateDirectoryParams creates a directory on the host filesystem.
type FsCreateDirectoryParams struct {
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

// FsCreateDirectoryResponse is the empty response from fs/createDirectory.
type FsCreateDirectoryResponse struct{}

// FsGetMetadataParams requests metadata for an absolute path.
type FsGetMetadataParams struct {
	Path string `json:"path"`
}

// FsGetMetadataResponse contains metadata for a filesystem path.
type FsGetMetadataResponse struct {
	CreatedAtMs  int64 `json:"createdAtMs"`
	IsDirectory  bool  `json:"isDirectory"`
	IsFile       bool  `json:"isFile"`
	ModifiedAtMs int64 `json:"modifiedAtMs"`
}

func (r *FsGetMetadataResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "createdAtMs", "isDirectory", "isFile", "modifiedAtMs"); err != nil {
		return err
	}
	type wire FsGetMetadataResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = FsGetMetadataResponse(decoded)
	return nil
}

// FsReadDirectoryParams lists direct child names for a directory.
type FsReadDirectoryParams struct {
	Path string `json:"path"`
}

// FsReadDirectoryEntry represents a single direct child entry.
type FsReadDirectoryEntry struct {
	FileName    string `json:"fileName"`
	IsDirectory bool   `json:"isDirectory"`
	IsFile      bool   `json:"isFile"`
}

// FsReadDirectoryResponse contains directory entries.
type FsReadDirectoryResponse struct {
	Entries []FsReadDirectoryEntry `json:"entries"`
}

func (r *FsReadDirectoryResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "entries"); err != nil {
		return err
	}
	type wire FsReadDirectoryResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = FsReadDirectoryResponse(decoded)
	return nil
}

// FsRemoveParams removes a file or directory tree from the host filesystem.
type FsRemoveParams struct {
	Force     *bool  `json:"force,omitempty"`
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

// FsRemoveResponse is the empty response from fs/remove.
type FsRemoveResponse struct{}

// FsCopyParams copies a file or directory tree on the host filesystem.
type FsCopyParams struct {
	DestinationPath string `json:"destinationPath"`
	Recursive       *bool  `json:"recursive,omitempty"`
	SourcePath      string `json:"sourcePath"`
}

// FsCopyResponse is the empty response from fs/copy.
type FsCopyResponse struct{}

// FsService provides host filesystem operations.
type FsService struct {
	client *Client
}

func newFsService(client *Client) *FsService {
	return &FsService{client: client}
}

// ReadFile reads a file from the host filesystem.
func (s *FsService) ReadFile(ctx context.Context, params FsReadFileParams) (FsReadFileResponse, error) {
	var resp FsReadFileResponse
	if err := s.client.sendRequest(ctx, methodFsReadFile, params, &resp); err != nil {
		return FsReadFileResponse{}, err
	}
	return resp, nil
}

// WriteFile writes a file on the host filesystem.
func (s *FsService) WriteFile(ctx context.Context, params FsWriteFileParams) (FsWriteFileResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodFsWriteFile, params); err != nil {
		return FsWriteFileResponse{}, err
	}
	return FsWriteFileResponse{}, nil
}

// CreateDirectory creates a directory on the host filesystem.
func (s *FsService) CreateDirectory(ctx context.Context, params FsCreateDirectoryParams) (FsCreateDirectoryResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodFsCreateDirectory, params); err != nil {
		return FsCreateDirectoryResponse{}, err
	}
	return FsCreateDirectoryResponse{}, nil
}

// GetMetadata retrieves metadata for a filesystem path.
func (s *FsService) GetMetadata(ctx context.Context, params FsGetMetadataParams) (FsGetMetadataResponse, error) {
	var resp FsGetMetadataResponse
	if err := s.client.sendRequest(ctx, methodFsGetMetadata, params, &resp); err != nil {
		return FsGetMetadataResponse{}, err
	}
	return resp, nil
}

// ReadDirectory lists direct child entries for a directory.
func (s *FsService) ReadDirectory(ctx context.Context, params FsReadDirectoryParams) (FsReadDirectoryResponse, error) {
	var resp FsReadDirectoryResponse
	if err := s.client.sendRequest(ctx, methodFsReadDirectory, params, &resp); err != nil {
		return FsReadDirectoryResponse{}, err
	}
	return resp, nil
}

// Remove removes a file or directory tree from the host filesystem.
func (s *FsService) Remove(ctx context.Context, params FsRemoveParams) (FsRemoveResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodFsRemove, params); err != nil {
		return FsRemoveResponse{}, err
	}
	return FsRemoveResponse{}, nil
}

// Copy copies a file or directory tree on the host filesystem.
func (s *FsService) Copy(ctx context.Context, params FsCopyParams) (FsCopyResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodFsCopy, params); err != nil {
		return FsCopyResponse{}, err
	}
	return FsCopyResponse{}, nil
}
