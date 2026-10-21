package dataimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ImportMode Constants Tests
// =============================================================================

func TestImportModeConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ImportMode("create"), ImportModeCreate)
	assert.Equal(t, ImportMode("update"), ImportModeUpdate)
}

// =============================================================================
// ErrorSeverity Constants Tests
// =============================================================================

func TestErrorSeverityConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ErrorSeverity("error"), ErrorSeverityError)
	assert.Equal(t, ErrorSeverity("warning"), ErrorSeverityWarning)
	assert.Equal(t, ErrorSeverity("info"), ErrorSeverityInfo)
}
