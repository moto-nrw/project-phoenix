package schoolstructure_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/stretchr/testify/assert"
)

func TestTransitionErrorCodesAreStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "not_found", schoolstructure.ErrorCode(schoolstructure.ErrTransitionNotFound))
	assert.Equal(t, "invalid", schoolstructure.ErrorCode(&schoolstructure.InvalidTransitionError{Reason: "x"}))
	assert.Equal(t, "conflict", schoolstructure.ErrorCode(schoolstructure.ErrTransitionStateConflict))
	assert.True(t, errors.Is(&schoolstructure.InvalidTransitionError{Reason: "x"}, schoolstructure.ErrInvalidTransition))
}
