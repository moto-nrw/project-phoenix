package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ParseLeadingGradeLevel pulls the leading integer out of a school-class
// string. The autofill flow uses it to default the form's
// target_grade_level to "current grade + 1" (the rollover-bumps-grade
// case for next year), so a zero return means "no prefill" — the
// parent must pick the grade manually. Shared by the public enrollment
// route and the parents-portal route.

func TestParseLeadingGradeLevel_DigitThenLetter(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, ParseLeadingGradeLevel("1a"))
	assert.Equal(t, 2, ParseLeadingGradeLevel("2b"))
	assert.Equal(t, 4, ParseLeadingGradeLevel("4c"))
}

func TestParseLeadingGradeLevel_AllDigits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10, ParseLeadingGradeLevel("10"))
	assert.Equal(t, 13, ParseLeadingGradeLevel("13"))
}

func TestParseLeadingGradeLevel_SupportedPrefixedLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 3, ParseLeadingGradeLevel("Klasse 3a"))
	assert.Equal(t, 1, ParseLeadingGradeLevel("a1"))

	// Labels without any numeric grade still produce no prefill.
	assert.Equal(t, 0, ParseLeadingGradeLevel("Vorschule"))
}

func TestParseLeadingGradeLevel_EmptyIsZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, ParseLeadingGradeLevel(""))
}

func TestParseLeadingGradeLevel_PurePunctuationIsZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, ParseLeadingGradeLevel("---"))
}

func TestParseLeadingGradeLevel_MultiDigitThenLetter(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 11, ParseLeadingGradeLevel("11x"))
	assert.Equal(t, 12, ParseLeadingGradeLevel("12z"))
}

func TestParseLeadingGradeLevel_StopsAtFirstNonDigit(t *testing.T) {
	t.Parallel()

	// "3-special" → 3. The function intentionally takes only the
	// leading run, not the digit-most-frequent.
	assert.Equal(t, 3, ParseLeadingGradeLevel("3-special"))
}
