package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorInvitationToken_Validate_Valid(t *testing.T) {
	token := &OperatorInvitationToken{
		Email:     "test@example.com",
		Token:     "abc-123-def",
		CreatedBy: 42,
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	err := token.Validate()
	assert.NoError(t, err)
}

func TestOperatorInvitationToken_Validate_MissingEmail(t *testing.T) {
	token := &OperatorInvitationToken{
		Token:     "abc-123-def",
		CreatedBy: 42,
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	err := token.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestOperatorInvitationToken_Validate_MissingToken(t *testing.T) {
	token := &OperatorInvitationToken{
		Email:     "test@example.com",
		CreatedBy: 42,
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	err := token.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token value is required")
}

func TestOperatorInvitationToken_Validate_MissingCreatedBy(t *testing.T) {
	token := &OperatorInvitationToken{
		Email:     "test@example.com",
		Token:     "abc-123-def",
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	err := token.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by operator ID is required")
}

func TestOperatorInvitationToken_Validate_NegativeCreatedBy(t *testing.T) {
	token := &OperatorInvitationToken{
		Email:     "test@example.com",
		Token:     "abc-123-def",
		CreatedBy: -1,
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	err := token.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by operator ID is required")
}

func TestOperatorInvitationToken_Validate_AlreadyUsed(t *testing.T) {
	usedAt := time.Now()
	token := &OperatorInvitationToken{
		Email:     "test@example.com",
		Token:     "abc-123-def",
		CreatedBy: 42,
		ExpiresAt: time.Now().Add(48 * time.Hour),
		UsedAt:    &usedAt,
	}

	err := token.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token has already been used")
}

func TestOperatorInvitationToken_IsUsed(t *testing.T) {
	t.Run("not used", func(t *testing.T) {
		token := &OperatorInvitationToken{}
		assert.False(t, token.IsUsed())
	})

	t.Run("used", func(t *testing.T) {
		usedAt := time.Now()
		token := &OperatorInvitationToken{UsedAt: &usedAt}
		assert.True(t, token.IsUsed())
	})
}

func TestOperatorInvitationToken_GetID(t *testing.T) {
	token := &OperatorInvitationToken{}
	token.ID = 42
	assert.Equal(t, int64(42), token.GetID())
}

func TestOperatorInvitationToken_GetCreatedAt(t *testing.T) {
	now := time.Now()
	token := &OperatorInvitationToken{}
	token.CreatedAt = now
	assert.Equal(t, now, token.GetCreatedAt())
}

func TestOperatorInvitationToken_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	token := &OperatorInvitationToken{}
	token.UpdatedAt = now
	assert.Equal(t, now, token.GetUpdatedAt())
}
