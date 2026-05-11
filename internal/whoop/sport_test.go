package whoop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSportName(t *testing.T) {
	require.Equal(t, "Powerlifting", resolveSportName(59, ""))
	require.Equal(t, "Powerlifting", resolveSportName(59, "powerlifting"),
		"local map should win over API-provided name")
	require.Equal(t, "Walking", resolveSportName(63, ""))
	require.Equal(t, "Activity", resolveSportName(-1, ""))
	require.Equal(t, "Running", resolveSportName(0, ""))

	// Unknown ID: fall back to API name.
	require.Equal(t, "Underwater Basket Weaving", resolveSportName(9999, "Underwater Basket Weaving"))

	// Unknown ID with no API name: generic.
	require.Equal(t, "Sport", resolveSportName(9999, ""))
}
