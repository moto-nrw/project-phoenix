package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseGradeLevel pulls the leading integer out of a school-class
// string. The autofill flow uses it to default the form's
// target_grade_level to "current grade + 1" (the rollover-bumps-grade
// case for next year), so a zero return means "no prefill" — the
// parent must pick the grade manually. Mirrors the parent portal's
// parseLeadingGrade helper one package over.

func TestParseGradeLevel_DigitThenLetter(t *testing.T) {
	assert.Equal(t, 1, parseGradeLevel("1a"))
	assert.Equal(t, 2, parseGradeLevel("2b"))
	assert.Equal(t, 4, parseGradeLevel("4c"))
}

func TestParseGradeLevel_AllDigits(t *testing.T) {
	assert.Equal(t, 10, parseGradeLevel("10"))
	assert.Equal(t, 13, parseGradeLevel("13"))
}

func TestParseGradeLevel_LeadingLetterIsZero(t *testing.T) {
	// "a1" → 0. "Vorschule" → 0. Both signal "no autofill" to the form.
	assert.Equal(t, 0, parseGradeLevel("a1"))
	assert.Equal(t, 0, parseGradeLevel("Vorschule"))
}

func TestParseGradeLevel_EmptyIsZero(t *testing.T) {
	assert.Equal(t, 0, parseGradeLevel(""))
}

func TestParseGradeLevel_PurePunctuationIsZero(t *testing.T) {
	assert.Equal(t, 0, parseGradeLevel("---"))
}

func TestParseGradeLevel_MultiDigitThenLetter(t *testing.T) {
	assert.Equal(t, 11, parseGradeLevel("11x"))
	assert.Equal(t, 12, parseGradeLevel("12z"))
}

func TestParseGradeLevel_StopsAtFirstNonDigit(t *testing.T) {
	// "3-special" → 3. The function intentionally takes only the
	// leading run, not the digit-most-frequent.
	assert.Equal(t, 3, parseGradeLevel("3-special"))
}
