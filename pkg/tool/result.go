package tool

import (
	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/model"
)

// OutputRef describes where tool output has been persisted when it is too large
// (or otherwise undesirable) to embed directly in ToolResult.Output.
type OutputRef struct {
	Path      string `json:"path,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Preview carries lightweight metadata that callers can use for UI summaries.
type Preview struct {
	Title     string `json:"title,omitempty"`
	Summary   string `json:"summary,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

// ToolResult captures the outcome of a tool invocation.
type ToolResult struct {
	Success       bool
	Output        string                 `json:"output,omitempty"`
	Summary       string                 `json:"summary,omitempty"`
	OutputRef     *OutputRef             `json:"output_ref,omitempty"`
	ContentBlocks []model.ContentBlock   `json:"content_blocks,omitempty"`
	Artifacts     []artifact.ArtifactRef `json:"artifacts,omitempty"`
	Structured    any                    `json:"structured,omitempty"`
	Preview       *Preview               `json:"preview,omitempty"`
	Data          interface{}            `json:"data,omitempty"`
	Error         error                  `json:"-"`
}
