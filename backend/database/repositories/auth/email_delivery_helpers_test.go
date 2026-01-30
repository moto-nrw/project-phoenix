package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxEmailErrorLength_ConstantValue(t *testing.T) {
	assert.Equal(t, 1024, maxEmailErrorLength)
}

func TestTruncateError_EmptyString(t *testing.T) {
	result := truncateError("")
	assert.Equal(t, "", result)
}

func TestTruncateError_ShortString(t *testing.T) {
	input := "short error message"
	result := truncateError(input)
	assert.Equal(t, input, result)
}

func TestTruncateError_ExactlyMaxLength(t *testing.T) {
	input := strings.Repeat("a", maxEmailErrorLength)
	result := truncateError(input)
	assert.Equal(t, input, result)
	assert.Len(t, result, maxEmailErrorLength)
}

func TestTruncateError_OneByteTooLong(t *testing.T) {
	input := strings.Repeat("a", maxEmailErrorLength+1)
	result := truncateError(input)
	assert.Len(t, result, maxEmailErrorLength)
	assert.Equal(t, strings.Repeat("a", maxEmailErrorLength), result)
}

func TestTruncateError_VeryLongString(t *testing.T) {
	input := strings.Repeat("x", 5000)
	result := truncateError(input)
	assert.Len(t, result, maxEmailErrorLength)
	assert.Equal(t, strings.Repeat("x", maxEmailErrorLength), result)
}

func TestTruncateError_PreservesPrefix(t *testing.T) {
	prefix := "ERROR: "
	input := prefix + strings.Repeat("a", 2000)
	result := truncateError(input)
	assert.Len(t, result, maxEmailErrorLength)
	assert.True(t, strings.HasPrefix(result, prefix))
}

func TestTruncateError_MultilineString(t *testing.T) {
	input := strings.Repeat("line\n", 300) // Creates ~1500 chars
	result := truncateError(input)
	assert.Len(t, result, maxEmailErrorLength)
}

func TestTruncateError_SpecialCharacters(t *testing.T) {
	input := strings.Repeat("🎉", maxEmailErrorLength/4+100) // Unicode chars
	result := truncateError(input)
	assert.LessOrEqual(t, len(result), maxEmailErrorLength)
}
