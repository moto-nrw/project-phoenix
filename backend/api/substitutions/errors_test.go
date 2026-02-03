package substitutions_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/substitutions"
	"github.com/stretchr/testify/assert"
)

func TestErrorVariablesExist(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrSubstitutionNotFound", substitutions.ErrSubstitutionNotFound},
		{"ErrSubstitutionConflict", substitutions.ErrSubstitutionConflict},
		{"ErrInvalidSubstitutionData", substitutions.ErrInvalidSubstitutionData},
		{"ErrSubstitutionDateRange", substitutions.ErrSubstitutionDateRange},
		{"ErrStaffAlreadySubstituting", substitutions.ErrStaffAlreadySubstituting},
		{"ErrGroupAlreadyHasSubstitute", substitutions.ErrGroupAlreadyHasSubstitute},
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
	errors := []error{
		substitutions.ErrSubstitutionNotFound,
		substitutions.ErrSubstitutionConflict,
		substitutions.ErrInvalidSubstitutionData,
		substitutions.ErrSubstitutionDateRange,
		substitutions.ErrStaffAlreadySubstituting,
		substitutions.ErrGroupAlreadyHasSubstitute,
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
	tests := []struct {
		name            string
		err             error
		expectedContain string
	}{
		{"ErrSubstitutionNotFound", substitutions.ErrSubstitutionNotFound, "not found"},
		{"ErrSubstitutionConflict", substitutions.ErrSubstitutionConflict, "conflict"},
		{"ErrInvalidSubstitutionData", substitutions.ErrInvalidSubstitutionData, "invalid"},
		{"ErrSubstitutionDateRange", substitutions.ErrSubstitutionDateRange, "date range"},
		{"ErrStaffAlreadySubstituting", substitutions.ErrStaffAlreadySubstituting, "already substituting"},
		{"ErrGroupAlreadyHasSubstitute", substitutions.ErrGroupAlreadyHasSubstitute, "already has"},
		{"ErrSubstitutionBackdated", substitutions.ErrSubstitutionBackdated, "past dates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.err.Error(), tt.expectedContain)
		})
	}
}
