package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeTagID_AlreadyNormalized(t *testing.T) {
	result := normalizeTagID("ABCD1234")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_WithColons(t *testing.T) {
	result := normalizeTagID("AB:CD:12:34")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_WithDashes(t *testing.T) {
	result := normalizeTagID("ab-cd-12-34")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_WithSpaces(t *testing.T) {
	result := normalizeTagID("ab cd 12 34")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_LeadingTrailingSpaces(t *testing.T) {
	result := normalizeTagID("  ABCD1234  ")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_EmptyString(t *testing.T) {
	result := normalizeTagID("")
	assert.Equal(t, "", result)
}

func TestNormalizeTagID_MixedSeparators(t *testing.T) {
	result := normalizeTagID("aB:cD-12 34")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_LowercaseToUppercase(t *testing.T) {
	result := normalizeTagID("abcd")
	assert.Equal(t, "ABCD", result)
}

func TestNormalizeTagID_OnlySpaces(t *testing.T) {
	result := normalizeTagID("   ")
	assert.Equal(t, "", result)
}

func TestNormalizeTagID_MultipleSeparatorsInRow(t *testing.T) {
	result := normalizeTagID("AB::CD--12  34")
	assert.Equal(t, "ABCD1234", result)
}

func TestNormalizeTagID_NumericOnly(t *testing.T) {
	result := normalizeTagID("12:34:56:78")
	assert.Equal(t, "12345678", result)
}

func TestNormalizeTagID_AlphabeticOnly(t *testing.T) {
	result := normalizeTagID("ab-cd-ef-gh")
	assert.Equal(t, "ABCDEFGH", result)
}

func TestNormalizeTagID_ComplexRealWorldCase(t *testing.T) {
	result := normalizeTagID("  1a:2b-3c 4d  ")
	assert.Equal(t, "1A2B3C4D", result)
}
