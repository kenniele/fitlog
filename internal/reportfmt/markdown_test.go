package reportfmt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscape(t *testing.T) {
	require.Equal(t, `a\_b\*c\!`, Escape("a_b*c!"))
	require.Equal(t, `12\.5`, Number(12.50, 1))
}

func TestPluralDays(t *testing.T) {
	require.Equal(t, "день", PluralDays(1))
	require.Equal(t, "дня", PluralDays(22))
	require.Equal(t, "дней", PluralDays(11))
	require.Equal(t, "дней", PluralDays(30))
}

func TestSplit(t *testing.T) {
	input := strings.Repeat("a", MessageLimit-10) + "\n\n" + strings.Repeat("b", 20)
	parts := Split(input)
	require.Len(t, parts, 2)
	for _, part := range parts {
		require.LessOrEqual(t, len(part), MessageLimit)
	}
}
