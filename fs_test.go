package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestFsReadFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"dataBase64":"ZGF0YQ=="}`),
		})

		resp, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "/tmp/file.txt"})
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if resp.DataBase64 != "ZGF0YQ==" {
			t.Fatalf("DataBase64 = %q; want ZGF0YQ==", resp.DataBase64)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "fs/readFile" {
			t.Fatalf("method = %q; want fs/readFile", req.Method)
		}
		var params codex.FsReadFileParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Path != "/tmp/file.txt" {
			t.Fatalf("Path = %q; want /tmp/file.txt", params.Path)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{JSONRPC: "2.0"})

		_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "/tmp/file.txt"})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("null result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`null`),
		})

		_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "/tmp/file.txt"})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("malformed result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"dataBase64":123}`),
		})

		_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "/tmp/file.txt"})
		if err == nil {
			t.Fatal("expected malformed result error")
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readFile", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Fs.ReadFile(context.Background(), codex.FsReadFileParams{Path: "/tmp/file.txt"})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestFsWriteFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)

		_, err := client.Fs.WriteFile(context.Background(), codex.FsWriteFileParams{
			Path:       "/tmp/file.txt",
			DataBase64: "ZGF0YQ==",
		})
		if err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "fs/writeFile" {
			t.Fatalf("method = %q; want fs/writeFile", req.Method)
		}
		var params codex.FsWriteFileParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Path != "/tmp/file.txt" || params.DataBase64 != "ZGF0YQ==" {
			t.Fatalf("params = %+v; want path and base64 payload preserved", params)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/writeFile", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInvalidRequest,
				Message: "denied",
			},
		})

		_, err := client.Fs.WriteFile(context.Background(), codex.FsWriteFileParams{
			Path:       "/tmp/file.txt",
			DataBase64: "ZGF0YQ==",
		})
		assertRPCErrorCode(t, err, codex.ErrCodeInvalidRequest)
	})
}

func TestFsCreateDirectory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		recursive := true

		_, err := client.Fs.CreateDirectory(context.Background(), codex.FsCreateDirectoryParams{
			Path:      "/tmp/dir",
			Recursive: &recursive,
		})
		if err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "fs/createDirectory" {
			t.Fatalf("method = %q; want fs/createDirectory", req.Method)
		}
		var params codex.FsCreateDirectoryParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Path != "/tmp/dir" || params.Recursive == nil || !*params.Recursive {
			t.Fatalf("params = %+v; want recursive directory creation payload", params)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/createDirectory", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Fs.CreateDirectory(context.Background(), codex.FsCreateDirectoryParams{Path: "/tmp/dir"})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestFsGetMetadata(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/getMetadata", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"createdAtMs":1,"isDirectory":false,"isFile":true,"isSymlink":false,"modifiedAtMs":2}`),
		})

		resp, err := client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file.txt"})
		if err != nil {
			t.Fatalf("GetMetadata() error = %v", err)
		}
		if resp.CreatedAtMs != 1 || !resp.IsFile || resp.IsDirectory || resp.ModifiedAtMs != 2 {
			t.Fatalf("response = %+v; want decoded metadata", resp)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/getMetadata", codex.Response{JSONRPC: "2.0"})

		_, err := client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file.txt"})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("malformed result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/getMetadata", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"createdAtMs":"bad","isDirectory":false,"isFile":true,"modifiedAtMs":2}`),
		})

		_, err := client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file.txt"})
		if err == nil {
			t.Fatal("expected malformed result error")
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/getMetadata", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"createdAtMs":1,"isDirectory":false,"modifiedAtMs":2}`),
		})

		_, err := client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file.txt"})
		if !errors.Is(err, codex.ErrMissingResultField) {
			t.Fatalf("error = %v; want ErrMissingResultField", err)
		}
	})

	t.Run("null required field", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/getMetadata", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"createdAtMs":1,"isDirectory":false,"isFile":null,"modifiedAtMs":2}`),
		})

		_, err := client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file.txt"})
		if !errors.Is(err, codex.ErrNullResultField) {
			t.Fatalf("error = %v; want ErrNullResultField", err)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/getMetadata", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Fs.GetMetadata(context.Background(), codex.FsGetMetadataParams{Path: "/tmp/file.txt"})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestFsReadDirectory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readDirectory", codex.Response{
			JSONRPC: "2.0",
			Result: json.RawMessage(`{
				"entries":[
					{"fileName":"a.txt","isDirectory":false,"isFile":true},
					{"fileName":"nested","isDirectory":true,"isFile":false}
				]
			}`),
		})

		resp, err := client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
		if err != nil {
			t.Fatalf("ReadDirectory() error = %v", err)
		}
		if len(resp.Entries) != 2 {
			t.Fatalf("entry count = %d; want 2", len(resp.Entries))
		}
		if resp.Entries[0].FileName != "a.txt" || !resp.Entries[0].IsFile {
			t.Fatalf("first entry = %+v; want decoded file entry", resp.Entries[0])
		}
	})

	t.Run("empty result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readDirectory", codex.Response{JSONRPC: "2.0"})

		_, err := client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("malformed result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readDirectory", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"entries":"bad"}`),
		})

		_, err := client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
		if err == nil {
			t.Fatal("expected malformed result error")
		}
	})

	t.Run("missing required entry field", func(t *testing.T) {
		tests := []struct {
			name  string
			entry string
		}{
			{name: "missing fileName", entry: `{"isDirectory":false,"isFile":true}`},
			{name: "missing isDirectory", entry: `{"fileName":"a.txt","isFile":true}`},
			{name: "missing isFile", entry: `{"fileName":"a.txt","isDirectory":false}`},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				transport := NewMockTransport()
				client := codex.NewClient(transport)
				transport.SetResponse("fs/readDirectory", codex.Response{
					JSONRPC: "2.0",
					Result:  json.RawMessage(`{"entries":[` + tt.entry + `]}`),
				})

				_, err := client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
				if !errors.Is(err, codex.ErrMissingResultField) {
					t.Fatalf("error = %v; want ErrMissingResultField", err)
				}
			})
		}
	})

	t.Run("null required entry field", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readDirectory", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"entries":[{"fileName":"a.txt","isDirectory":false,"isFile":null}]}`),
		})

		_, err := client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
		if !errors.Is(err, codex.ErrNullResultField) {
			t.Fatalf("error = %v; want ErrNullResultField", err)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/readDirectory", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Fs.ReadDirectory(context.Background(), codex.FsReadDirectoryParams{Path: "/tmp"})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestFsRemove(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		force := true
		recursive := true

		_, err := client.Fs.Remove(context.Background(), codex.FsRemoveParams{
			Path:      "/tmp/file.txt",
			Force:     &force,
			Recursive: &recursive,
		})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "fs/remove" {
			t.Fatalf("method = %q; want fs/remove", req.Method)
		}
		var params codex.FsRemoveParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.Path != "/tmp/file.txt" || params.Force == nil || !*params.Force || params.Recursive == nil || !*params.Recursive {
			t.Fatalf("params = %+v; want remove payload preserved", params)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/remove", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Fs.Remove(context.Background(), codex.FsRemoveParams{Path: "/tmp/file.txt"})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestFsCopy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		recursive := true

		_, err := client.Fs.Copy(context.Background(), codex.FsCopyParams{
			SourcePath:      "/tmp/src",
			DestinationPath: "/tmp/dst",
			Recursive:       &recursive,
		})
		if err != nil {
			t.Fatalf("Copy() error = %v", err)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "fs/copy" {
			t.Fatalf("method = %q; want fs/copy", req.Method)
		}
		var params codex.FsCopyParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.SourcePath != "/tmp/src" || params.DestinationPath != "/tmp/dst" || params.Recursive == nil || !*params.Recursive {
			t.Fatalf("params = %+v; want copy payload preserved", params)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("fs/copy", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Fs.Copy(context.Background(), codex.FsCopyParams{
			SourcePath:      "/tmp/src",
			DestinationPath: "/tmp/dst",
		})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}
