package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOperatorInvitationToken_GetID(t *testing.T) {
	t.Parallel()

	token := &OperatorInvitationToken{}
	token.ID = 42
	assert.Equal(t, int64(42), token.GetID())
}

func TestOperatorInvitationToken_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &OperatorInvitationToken{}
	token.CreatedAt = now
	assert.Equal(t, now, token.GetCreatedAt())
}

func TestOperatorInvitationToken_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &OperatorInvitationToken{}
	token.UpdatedAt = now
	assert.Equal(t, now, token.GetUpdatedAt())
}
