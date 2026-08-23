package substitutions_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/substitutions"
	"github.com/stretchr/testify/assert"
)

func TestErrorVariablesExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrSubstitutionNotFound", substitutions.ErrSubstitutionNotFound},
		{"ErrInvalidSubstitutionData", substitutions.ErrInvalidSubstitutionData},
		{"ErrSubstitutionDateRange", substitutions.ErrSubstitutionDateRange},
		{"ErrStaffAlreadySubstituting", substitutions.ErrStaffAlreadySubstituting},
		{"ErrSubstitutionBackdated", substitutions.ErrSubstitutionBackdated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

func TestErrorVariablesDistinct(t *testing.T) {
	t.Parallel()

	errors := []error{
		substitutions.ErrSubstitutionNotFound,
		substitutions.ErrInvalidSubstitutionData,
		substitutions.ErrSubstitutionDateRange,
		substitutions.ErrStaffAlreadySubstituting,
		substitutions.ErrSubstitutionBackdated,
	}

	messages := make(map[string]bool)
	for _, err := range errors {
		msg := err.Error()
		assert.False(t, messages[msg], "duplicate error message: %s", msg)
		messages[msg] = true
	}
}

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		err             error
		expectedContain string
	}{
		{"ErrSubstitutionNotFound", substitutions.ErrSubstitutionNotFound, "not found"},
		{"ErrInvalidSubstitutionData", substitutions.ErrInvalidSubstitutionData, "invalid"},
		{"ErrSubstitutionDateRange", substitutions.ErrSubstitutionDateRange, "date range"},
		{"ErrStaffAlreadySubstituting", substitutions.ErrStaffAlreadySubstituting, "already substituting"},
		{"ErrSubstitutionBackdated", substitutions.ErrSubstitutionBackdated, "past dates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.err.Error(), tt.expectedContain)
		})
	}
}
