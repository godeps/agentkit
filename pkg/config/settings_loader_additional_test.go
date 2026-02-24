package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingsPathHelpers(t *testing.T) {
	require.Equal(t, "", getProjectSettingsPath(""))
	require.Equal(t, "", getLocalSettingsPath(""))
}
