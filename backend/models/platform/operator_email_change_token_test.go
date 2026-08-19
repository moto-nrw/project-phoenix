package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorEmailChangeToken_Validate(t *testing.T) {
	t.Parallel()

	validToken := func() *OperatorEmailChangeToken {
		return &OperatorEmailChangeToken{
			OperatorID: 42,
			NewEmail:   "new@example.com",
			Token:      "abc-123-def",
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
	}

	t.Run("Valid", func(t *testing.T) {
		token := validToken()
		err := token.Validate()
		require.NoError(t, err)
	})

	t.Run("MissingOperatorID", func(t *testing.T) {
		token := validToken()
		token.OperatorID = 0
		err := token.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operator ID is required")
	})

	t.Run("EmptyTokenString", func(t *testing.T) {
		token := validToken()
		token.Token = ""
		err := token.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token value is required")
	})

	t.Run("EmptyNewEmail", func(t *testing.T) {
		token := validToken()
		token.NewEmail = ""
		err := token.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "new email is required")
	})

	t.Run("UsedToken", func(t *testing.T) {
		token := validToken()
		token.Used = true
		err := token.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already been used")
	})
}

func TestOperatorEmailChangeToken_Accessors(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &OperatorEmailChangeToken{}
	token.ID = 42
	token.CreatedAt = now
	token.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(42), token.GetID())
	assert.Equal(t, now, token.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), token.GetUpdatedAt())
}
