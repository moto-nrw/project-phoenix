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

func TestOperatorInvitationToken_Validate_ExpiredToken(t *testing.T) {
	token := &OperatorInvitationToken{
		Email:     "test@example.com",
		Token:     "abc-123-def",
		CreatedBy: 42,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	err := token.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token has already expired")
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

func TestOperatorInvitationToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "just expired",
			expiresAt: time.Now().Add(-1 * time.Millisecond),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OperatorInvitationToken{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, token.IsExpired())
		})
	}
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

func TestOperatorInvitationToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		usedAt    *time.Time
		want      bool
	}{
		{
			name:      "valid - not expired, not used",
			expiresAt: time.Now().Add(1 * time.Hour),
			usedAt:    nil,
			want:      true,
		},
		{
			name:      "invalid - expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			usedAt:    nil,
			want:      false,
		},
		{
			name:      "invalid - used",
			expiresAt: time.Now().Add(1 * time.Hour),
			usedAt:    ptrTime(time.Now()),
			want:      false,
		},
		{
			name:      "invalid - expired and used",
			expiresAt: time.Now().Add(-1 * time.Hour),
			usedAt:    ptrTime(time.Now()),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OperatorInvitationToken{
				ExpiresAt: tt.expiresAt,
				UsedAt:    tt.usedAt,
			}
			assert.Equal(t, tt.want, token.IsValid())
		})
	}
}

func TestOperatorInvitationToken_TableName(t *testing.T) {
	token := &OperatorInvitationToken{}
	assert.Equal(t, "platform.operator_invitation_tokens", token.TableName())
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

func ptrTime(t time.Time) *time.Time {
	return &t
}
