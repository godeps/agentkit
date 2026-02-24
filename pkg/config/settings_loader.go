package config

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// SettingsLoader composes settings using the simplified precedence model.
// Higher-priority layers override lower ones while preserving unspecified fields.
// Order (low -> high): defaults < project < local < runtime overrides.
type SettingsLoader struct {
	ProjectRoot      string
	RuntimeOverrides *Settings
	FS               *FS
}

// Load resolves and merges settings across all layers.
func (l *SettingsLoader) Load() (*Settings, error) {
	if strings.TrimSpace(l.ProjectRoot) == "" {
		return nil, errors.New("project root is required for settings loading")
	}

	root := l.ProjectRoot
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	} else {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}

	merged := GetDefaultSettings()

	layers := []struct {
		name string
		path string
	}{
		{name: "project", path: getProjectSettingsPath(root)},
		{name: "local", path: getLocalSettingsPath(root)},
	}

	for _, layer := range layers {
		if err := applySettingsLayer(&merged, layer.name, layer.path, l.FS); err != nil {
			return nil, err
		}
	}

	if l.RuntimeOverrides != nil {
		log.Printf("settings: applying runtime overrides")
		if next := MergeSettings(&merged, l.RuntimeOverrides); next != nil {
			merged = *next
		}
	} else {
		log.Printf("settings: no runtime overrides provided")
	}

	return &merged, nil
}

// getProjectSettingsPath returns the tracked project settings path.
func getProjectSettingsPath(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return filepath.Join(root, ".claude", "settings.json")
}

// getLocalSettingsPath returns the untracked project-local settings path.
func getLocalSettingsPath(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return filepath.Join(root, ".claude", "settings.local.json")
}

// loadJSONFile decodes a settings JSON file. Missing files return (nil, nil).
func loadJSONFile(path string, filesystem *FS) (*Settings, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	var (
		data []byte
		err  error
	)
	if filesystem != nil {
		data, err = filesystem.ReadFile(path)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &s, nil
}

func applySettingsLayer(dst *Settings, name, path string, filesystem *FS) error {
	if path == "" {
		log.Printf("settings: %s layer skipped (no path)", name)
		return nil
	}
	cfg, err := loadJSONFile(path, filesystem)
	if err != nil {
		return fmt.Errorf("load %s settings: %w", name, err)
	}
	if cfg == nil {
		log.Printf("settings: %s layer not found at %s", name, path)
		return nil
	}
	log.Printf("settings: applying %s layer from %s", name, path)
	if next := MergeSettings(dst, cfg); next != nil {
		*dst = *next
	}
	return nil
}
