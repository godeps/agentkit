package artifact

import (
	"encoding/json"
	"testing"
)

func TestArtifactIdentityAndKindClassification(t *testing.T) {
	art := Artifact{
		ID:   "art_123",
		Kind: ArtifactKindImage,
		Meta: ArtifactMeta{
			MediaType: "image/png",
		},
	}

	if art.ID != "art_123" {
		t.Fatalf("expected artifact ID to be preserved, got %q", art.ID)
	}
	if art.Kind != ArtifactKindImage {
		t.Fatalf("expected image artifact kind, got %q", art.Kind)
	}
	if art.Meta.MediaType != "image/png" {
		t.Fatalf("expected media type to be preserved, got %q", art.Meta.MediaType)
	}
}

func TestArtifactRefConstructors(t *testing.T) {
	local := NewLocalFileRef("/tmp/report.pdf", ArtifactKindDocument)
	if local.Path != "/tmp/report.pdf" {
		t.Fatalf("expected local path, got %+v", local)
	}
	if local.Kind != ArtifactKindDocument {
		t.Fatalf("expected document kind for local ref, got %q", local.Kind)
	}
	if local.Source != ArtifactSourceLocal {
		t.Fatalf("expected local source, got %q", local.Source)
	}

	remote := NewURLRef("https://example.com/image.png", ArtifactKindImage)
	if remote.URL != "https://example.com/image.png" {
		t.Fatalf("expected remote URL, got %+v", remote)
	}
	if remote.Source != ArtifactSourceURL {
		t.Fatalf("expected URL source, got %q", remote.Source)
	}

	generated := NewGeneratedRef("art_456", ArtifactKindAudio)
	if generated.ArtifactID != "art_456" {
		t.Fatalf("expected generated artifact ID, got %+v", generated)
	}
	if generated.Source != ArtifactSourceGenerated {
		t.Fatalf("expected generated source, got %q", generated.Source)
	}
}

func TestArtifactMetadataFields(t *testing.T) {
	meta := ArtifactMeta{
		MediaType: "audio/mpeg",
		SizeBytes: 4096,
		Checksum:  "sha256:abc123",
		Origin:    "tool:transcode",
	}

	if meta.MediaType != "audio/mpeg" {
		t.Fatalf("expected media type to round-trip, got %q", meta.MediaType)
	}
	if meta.SizeBytes != 4096 {
		t.Fatalf("expected size bytes to round-trip, got %d", meta.SizeBytes)
	}
	if meta.Checksum != "sha256:abc123" {
		t.Fatalf("expected checksum to round-trip, got %q", meta.Checksum)
	}
	if meta.Origin != "tool:transcode" {
		t.Fatalf("expected origin to round-trip, got %q", meta.Origin)
	}
}

func TestArtifactJSONTransportSafety(t *testing.T) {
	original := Artifact{
		ID:   "art_json",
		Kind: ArtifactKindDocument,
		Ref: ArtifactRef{
			Source: ArtifactSourceGenerated,
			Path:   "outputs/summary.pdf",
		},
		Meta: ArtifactMeta{
			MediaType: "application/pdf",
			SizeBytes: 1024,
			Checksum:  "sha256:def456",
			Origin:    "skill:summary",
		},
	}

	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}

	var decoded Artifact
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}

	if decoded != original {
		t.Fatalf("expected artifact JSON round-trip to preserve fields, got %+v want %+v", decoded, original)
	}
}
