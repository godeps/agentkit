package toolbuiltin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/security"
	"github.com/godeps/agentkit/pkg/tool"
)

const imageReadDescription = `Reads an image file from the local filesystem within the configured sandbox.
Use this when the model needs to inspect an image file as multimodal input.

Usage:
- The file_path parameter can be absolute or relative to the sandbox root
- Supported formats: png, jpeg, gif, webp, bmp
- The tool returns a text summary plus one image content block
- This tool reads image files only; use file_read for text files.`

var imageReadSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"file_path": map[string]any{
			"type":        "string",
			"description": "The absolute path to the image file to read",
		},
	},
	Required: []string{"file_path"},
}

var supportedImageMediaTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
	"image/webp": {},
	"image/bmp":  {},
}

type ImageReadTool struct {
	base *fileSandbox
}

func NewImageReadTool() *ImageReadTool {
	return NewImageReadToolWithRoot("")
}

func NewImageReadToolWithRoot(root string) *ImageReadTool {
	return &ImageReadTool{base: newFileSandbox(root)}
}

func NewImageReadToolWithSandbox(root string, sandbox *security.Sandbox) *ImageReadTool {
	return &ImageReadTool{base: newFileSandboxWithSandbox(root, sandbox)}
}

func (i *ImageReadTool) Name() string { return "ImageRead" }

func (i *ImageReadTool) Description() string { return imageReadDescription }

func (i *ImageReadTool) Schema() *tool.JSONSchema { return imageReadSchema }

func (i *ImageReadTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if i == nil || i.base == nil || i.base.sandbox == nil {
		return nil, errors.New("image_read tool is not initialised")
	}
	path, err := i.base.resolvePath(params["file_path"])
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}
	if i.base.maxBytes > 0 && info.Size() > i.base.maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes limit", i.base.maxBytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("image file is empty")
	}
	if i.base.maxBytes > 0 && int64(len(data)) > i.base.maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes limit", i.base.maxBytes)
	}

	mediaType := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	if semi := strings.Index(mediaType, ";"); semi >= 0 {
		mediaType = strings.TrimSpace(mediaType[:semi])
	}
	if _, ok := supportedImageMediaTypes[mediaType]; !ok {
		return nil, fmt.Errorf("unsupported image media type %q", mediaType)
	}

	display := displayPath(path, i.base.root)
	return &tool.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Image loaded: %s (%d bytes, %s)", display, len(data), mediaType),
		ContentBlocks: []model.ContentBlock{{
			Type:      model.ContentBlockImage,
			MediaType: mediaType,
			Data:      base64.StdEncoding.EncodeToString(data),
		}},
		Data: map[string]any{
			"path":       display,
			"media_type": mediaType,
			"size_bytes": len(data),
		},
	}, nil
}
