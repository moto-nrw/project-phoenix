package application

import (
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/stretchr/testify/require"
)

func TestMatchSnippetKeepsTheMatchInABoundedText(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Holt die Oma ab.", matchSnippet("  Holt die Oma ab. ", "oma"), "a short note stays whole")

	long := strings.Repeat("Vorher ein Satz. ", 12) + "Darf mit Großmutter Müller gehen. " + strings.Repeat("Danach ein Satz. ", 12)
	for _, query := range []string{"grossmutter muller", "MÜLLER"} {
		snippet := matchSnippet(long, query)
		require.Contains(t, snippet, "Müller", query)
		require.LessOrEqual(t, len([]rune(snippet)), schoolsearchview.MaxMatchTextRunes, query)
		require.True(t, strings.HasPrefix(snippet, "…Vorher") || strings.HasPrefix(snippet, "…ein") || strings.HasPrefix(snippet, "…Satz"), snippet)
		require.True(t, strings.HasSuffix(snippet, "…"), snippet)
	}

	unplaced := matchSnippet(long, "nirgends")
	require.True(t, strings.HasPrefix(unplaced, "Vorher ein Satz."), "a match the folding cannot place shows the start")
	require.True(t, strings.HasSuffix(unplaced, "…"))
	require.LessOrEqual(t, len([]rune(unplaced)), schoolsearchview.MaxMatchTextRunes)
}
