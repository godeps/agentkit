package artifact

import "testing"

func TestLineageGraphRecordsDerivedArtifacts(t *testing.T) {
	src := NewLocalFileRef("/tmp/source.png", ArtifactKindImage)
	derived := NewGeneratedRef("art_thumb", ArtifactKindImage)

	var graph LineageGraph
	graph.AddEdge(src, derived, "thumbnail")

	if len(graph.Edges) != 1 {
		t.Fatalf("expected one lineage edge, got %+v", graph.Edges)
	}
	if graph.Edges[0].Operation != "thumbnail" {
		t.Fatalf("expected operation to be preserved, got %+v", graph.Edges[0])
	}
	if got := graph.ChildrenOf(src); len(got) != 1 || got[0] != derived {
		t.Fatalf("expected derived child to be discoverable, got %+v", got)
	}
}

func TestLineageGraphPreservesProvenanceAcrossMultiStepPipeline(t *testing.T) {
	raw := NewLocalFileRef("/tmp/raw.mov", ArtifactKindVideo)
	audio := NewGeneratedRef("art_audio", ArtifactKindAudio)
	transcript := NewGeneratedRef("art_text", ArtifactKindText)

	var graph LineageGraph
	graph.AddEdge(raw, audio, "extract-audio")
	graph.AddEdge(audio, transcript, "transcribe")

	ancestors := graph.AncestorsOf(transcript)
	if len(ancestors) != 2 {
		t.Fatalf("expected full provenance chain, got %+v", ancestors)
	}
	if ancestors[0] != audio || ancestors[1] != raw {
		t.Fatalf("expected nearest-to-root ancestry ordering, got %+v", ancestors)
	}
}
