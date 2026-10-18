package workforce

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidSubstitutionPeriodKeepsCodeAndMessage(t *testing.T) {
	t.Parallel()
	cause := errors.New("parsing time")
	err := InvalidSubstitutionPeriod(cause)
	require.ErrorIs(t, err, ErrSubstitutionInvalidPeriod)
	assert.Equal(t, "substitutions.invalid_period", err.Code)
	assert.Equal(t, "Der Zeitraum ist ungültig.", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestMissingPersonnelNumbersCountsAbsentAndEmpty(t *testing.T) {
	t.Parallel()
	empty, number := "", "4711"
	assert.Equal(t, 2, MissingPersonnelNumbers([]*string{nil, &empty, &number}))
	assert.Zero(t, MissingPersonnelNumbers(nil))
}
