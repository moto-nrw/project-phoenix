package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuardianFailureKindMapsUnknownInvitationFailuresToInternal(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "internal", string(guardianFailureKind("")))
	assert.Equal(t, "forbidden", string(guardianFailureKind("forbidden")))
	assert.Equal(t, "invalid_request", string(guardianFailureKind("invalid_request")))
}
