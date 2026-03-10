package toolbuiltin

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestImageReadToolReadsPNG(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pixel.png")
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jkO8AAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}

	tool := NewImageReadToolWithRoot(root)
	res, err := tool.Execute(context.Background(), map[string]any{"file_path": "pixel.png"})
	if err != nil {
		t.Fatalf("execute image_read: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success result, got %+v", res)
	}
	if len(res.ContentBlocks) != 1 {
		t.Fatalf("expected one content block, got %+v", res.ContentBlocks)
	}
	if res.ContentBlocks[0].MediaType != "image/png" {
		t.Fatalf("unexpected media type: %+v", res.ContentBlocks[0])
	}
	if res.ContentBlocks[0].Data == "" {
		t.Fatalf("expected base64 data, got %+v", res.ContentBlocks[0])
	}
}

func TestImageReadToolRejectsNonImage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	tool := NewImageReadToolWithRoot(root)
	if _, err := tool.Execute(context.Background(), map[string]any{"file_path": "note.txt"}); err == nil {
		t.Fatal("expected non-image file to be rejected")
	}
}

func TestImageReadToolRejectsPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	tool := NewImageReadToolWithRoot(root)
	if _, err := tool.Execute(context.Background(), map[string]any{"file_path": "../outside.png"}); err == nil {
		t.Fatal("expected sandbox path validation error")
	}
}
