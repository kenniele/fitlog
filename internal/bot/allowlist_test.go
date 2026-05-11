package bot

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllowlist(t *testing.T) {
	a := NewAllowlist([]int64{42, 100}, slog.Default())
	require.True(t, a.Allowed(42))
	require.True(t, a.Allowed(100))
	require.False(t, a.Allowed(7))
	require.False(t, a.Allowed(0))
}
