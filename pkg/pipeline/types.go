package pipeline

import "github.com/godeps/agentkit/pkg/artifact"

// Step is the basic unit of multimodal pipeline declaration.
type Step struct {
	Name        string                 `json:"name,omitempty"`
	Tool        string                 `json:"tool,omitempty"`
	Skill       string                 `json:"skill,omitempty"`
	Input       []artifact.ArtifactRef `json:"input,omitempty"`
	With        map[string]any         `json:"with,omitempty"`
	Batch       *Batch                 `json:"batch,omitempty"`
	FanOut      *FanOut                `json:"fan_out,omitempty"`
	FanIn       *FanIn                 `json:"fan_in,omitempty"`
	Conditional *Conditional           `json:"conditional,omitempty"`
	Retry       *Retry                 `json:"retry,omitempty"`
	Checkpoint  *Checkpoint            `json:"checkpoint,omitempty"`
}

// Batch groups an ordered list of steps.
type Batch struct {
	Steps []Step `json:"steps,omitempty"`
}

// FanOut applies the same step across a named artifact collection.
type FanOut struct {
	Collection string `json:"collection,omitempty"`
	Step       Step   `json:"step"`
}

// FanIn aggregates fan-out results into a named target.
type FanIn struct {
	Strategy string `json:"strategy,omitempty"`
	Into     string `json:"into,omitempty"`
}

// Conditional chooses between two branches based on a runtime condition.
type Conditional struct {
	Condition string `json:"condition,omitempty"`
	Then      Step   `json:"then"`
	Else      *Step  `json:"else,omitempty"`
}

// Retry wraps a step with a bounded retry policy.
type Retry struct {
	Attempts int  `json:"attempts,omitempty"`
	Step     Step `json:"step"`
}

// Checkpoint marks a resumable boundary around a step.
type Checkpoint struct {
	Name string `json:"name,omitempty"`
	Step Step   `json:"step"`
}
