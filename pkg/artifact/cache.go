package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// CacheKey identifies a deterministic artifact-processing result.
type CacheKey string

// NewCacheKey derives a stable cache key from tool name, params, and input artifacts.
func NewCacheKey(tool string, params map[string]any, refs []ArtifactRef) CacheKey {
	payload := struct {
		Tool   string        `json:"tool"`
		Params any           `json:"params,omitempty"`
		Refs   []ArtifactRef `json:"refs,omitempty"`
	}{
		Tool:   tool,
		Params: normalizeValue(params),
		Refs:   normalizeRefs(refs),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return CacheKey(fmt.Sprintf("invalid:%s", tool))
	}
	sum := sha256.Sum256(raw)
	return CacheKey(hex.EncodeToString(sum[:]))
}

func normalizeRefs(refs []ArtifactRef) []ArtifactRef {
	if len(refs) == 0 {
		return nil
	}
	out := append([]ArtifactRef(nil), refs...)
	sort.Slice(out, func(i, j int) bool {
		return refIdentity(out[i]) < refIdentity(out[j])
	})
	return out
}

func refIdentity(ref ArtifactRef) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", ref.Source, ref.Kind, ref.ArtifactID, ref.Path, ref.URL)
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make([]struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		}, 0, len(keys))
		for _, key := range keys {
			ordered = append(ordered, struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
			}{
				Key:   key,
				Value: normalizeValue(val[key]),
			})
		}
		return ordered
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return val
	}
}
