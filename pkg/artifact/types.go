package artifact

// ArtifactKind classifies the logical type of an artifact.
type ArtifactKind string

const (
	ArtifactKindImage    ArtifactKind = "image"
	ArtifactKindDocument ArtifactKind = "document"
	ArtifactKindAudio    ArtifactKind = "audio"
	ArtifactKindVideo    ArtifactKind = "video"
	ArtifactKindText     ArtifactKind = "text"
	ArtifactKindJSON     ArtifactKind = "json"
	ArtifactKindBinary   ArtifactKind = "binary"
)

// ArtifactSource identifies how an artifact is referenced.
type ArtifactSource string

const (
	ArtifactSourceLocal     ArtifactSource = "local"
	ArtifactSourceURL       ArtifactSource = "url"
	ArtifactSourceGenerated ArtifactSource = "generated"
)

// ArtifactMeta stores transport-safe metadata about an artifact.
type ArtifactMeta struct {
	MediaType string `json:"media_type,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Checksum  string `json:"checksum,omitempty"`
	Origin    string `json:"origin,omitempty"`
}

// ArtifactRef points at the physical or logical location of an artifact.
type ArtifactRef struct {
	Source     ArtifactSource `json:"source"`
	Path       string         `json:"path,omitempty"`
	URL        string         `json:"url,omitempty"`
	ArtifactID string         `json:"artifact_id,omitempty"`
	Kind       ArtifactKind   `json:"kind,omitempty"`
}

// Artifact is the first-class runtime representation of a multimodal object.
type Artifact struct {
	ID   string       `json:"id"`
	Kind ArtifactKind `json:"kind"`
	Ref  ArtifactRef  `json:"ref"`
	Meta ArtifactMeta `json:"meta,omitempty"`
}

// NewLocalFileRef references an artifact stored on the local filesystem.
func NewLocalFileRef(path string, kind ArtifactKind) ArtifactRef {
	return ArtifactRef{
		Source: ArtifactSourceLocal,
		Path:   path,
		Kind:   kind,
	}
}

// NewURLRef references an artifact available at a remote URL.
func NewURLRef(rawURL string, kind ArtifactKind) ArtifactRef {
	return ArtifactRef{
		Source: ArtifactSourceURL,
		URL:    rawURL,
		Kind:   kind,
	}
}

// NewGeneratedRef references an artifact produced during runtime execution.
func NewGeneratedRef(id string, kind ArtifactKind) ArtifactRef {
	return ArtifactRef{
		Source:     ArtifactSourceGenerated,
		ArtifactID: id,
		Kind:       kind,
	}
}
