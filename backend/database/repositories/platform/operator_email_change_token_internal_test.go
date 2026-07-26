package platform

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/stretchr/testify/assert"
)

func TestTruncateEmailChangeError_EmptyString(t *testing.T) {
	assert.Equal(t, "", strutil.TruncateRunes("", maxEmailChangeErrorLength, ""))
}

func TestTruncateEmailChangeError_ShortString(t *testing.T) {
	assert.Equal(t, "short error", strutil.TruncateRunes("short error", maxEmailChangeErrorLength, ""))
}

func TestTruncateEmailChangeError_ExactlyMaxLength(t *testing.T) {
	s := strings.Repeat("a", maxEmailChangeErrorLength)
	assert.Equal(t, s, strutil.TruncateRunes(s, maxEmailChangeErrorLength, ""))
}

func TestTruncateEmailChangeError_OneByteTooLong(t *testing.T) {
	s := strings.Repeat("b", maxEmailChangeErrorLength+1)
	result := strutil.TruncateRunes(s, maxEmailChangeErrorLength, "")
	assert.Equal(t, maxEmailChangeErrorLength, len(result))
}

func TestTruncateEmailChangeError_UTF8Safety(t *testing.T) {
	// Multi-byte runes: each ä is 2 bytes in UTF-8
	s := strings.Repeat("ä", maxEmailChangeErrorLength+10)
	result := strutil.TruncateRunes(s, maxEmailChangeErrorLength, "")
	// Should truncate by rune count, not byte count
	runes := []rune(result)
	assert.Equal(t, maxEmailChangeErrorLength, len(runes))
}
