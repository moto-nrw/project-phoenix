package platform

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateEmailChangeError_EmptyString(t *testing.T) {
	assert.Equal(t, "", truncateEmailChangeError(""))
}

func TestTruncateEmailChangeError_ShortString(t *testing.T) {
	assert.Equal(t, "short error", truncateEmailChangeError("short error"))
}

func TestTruncateEmailChangeError_ExactlyMaxLength(t *testing.T) {
	s := strings.Repeat("a", maxEmailChangeErrorLength)
	assert.Equal(t, s, truncateEmailChangeError(s))
}

func TestTruncateEmailChangeError_OneByteTooLong(t *testing.T) {
	s := strings.Repeat("b", maxEmailChangeErrorLength+1)
	result := truncateEmailChangeError(s)
	assert.Equal(t, maxEmailChangeErrorLength, len(result))
}

func TestTruncateEmailChangeError_UTF8Safety(t *testing.T) {
	// Multi-byte runes: each ä is 2 bytes in UTF-8
	s := strings.Repeat("ä", maxEmailChangeErrorLength+10)
	result := truncateEmailChangeError(s)
	// Should truncate by rune count, not byte count
	runes := []rune(result)
	assert.Equal(t, maxEmailChangeErrorLength, len(runes))
}
