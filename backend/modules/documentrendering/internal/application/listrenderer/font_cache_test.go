package listexport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDocumentsReuseParsedFontCoverage(t *testing.T) {
	t.Parallel()
	first, err := newLetterChrome("first")
	require.NoError(t, err)
	second, err := newPageChrome(false, "second", "", time.Time{}, nil, "")
	require.NoError(t, err)
	require.True(t, first.primaryCov.font == second.primaryCov.font, "primary coverage parsed once")
	require.True(t, first.fallbackCov.font == second.fallbackCov.font, "fallback coverage parsed once")
	require.NotSame(t, first.primaryCov, second.primaryCov, "sfnt scratch buffers belong to each document")
	require.NotSame(t, first.fallbackCov, second.fallbackCov)
}
