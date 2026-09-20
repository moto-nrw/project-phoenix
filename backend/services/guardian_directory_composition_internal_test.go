package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyGuardianInvitationFailureClassifiesUnexpectedErrorsAsInternal(t *testing.T) {
	t.Parallel()

	assert.Equal(t, GuardianFailureInternal,
		ClassifyGuardianInvitationFailure(errors.New("invitation store unavailable")))
}
