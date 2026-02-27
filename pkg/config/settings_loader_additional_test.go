package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingsPathHelpers(t *testing.T) {
	require.Equal(t, "", getProjectSettingsPath("", ""))
	require.Equal(t, "", getLocalSettingsPath("", ""))
}

func TestSettingsLoaderConfigRoot(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	require.NoError(t, os.MkdirAll(configRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configRoot, "settings.json"), []byte(`{"model":"x"}`), 0o600))

	loader := SettingsLoader{ProjectRoot: root, ConfigRoot: "config"}
	got, err := loader.Load()
	require.NoError(t, err)
	require.Equal(t, "x", got.Model)
}
