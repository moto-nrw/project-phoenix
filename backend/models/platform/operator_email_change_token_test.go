package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorEmailChangeToken_Validate(t *testing.T) {
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

func TestOperatorEmailChangeToken_TableName(t *testing.T) {
	token := &OperatorEmailChangeToken{}
	assert.Equal(t, "platform.operator_email_change_tokens", token.TableName())
}

func TestOperatorEmailChangeToken_Accessors(t *testing.T) {
	now := time.Now()
	token := &OperatorEmailChangeToken{}
	token.ID = 42
	token.CreatedAt = now
	token.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(42), token.GetID())
	assert.Equal(t, now, token.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), token.GetUpdatedAt())
}

func TestOperatorEmailChangeToken_BeforeAppendModel(t *testing.T) {
	token := &OperatorEmailChangeToken{}

	t.Run("UnknownQueryType", func(t *testing.T) {
		// Non-Update/Delete query types should be silently ignored.
		// Pass a string (not a *bun.UpdateQuery or *bun.DeleteQuery) to
		// verify the type-switch falls through without error or panic.
		err := token.BeforeAppendModel("not a query")
		assert.NoError(t, err)
	})

	t.Run("NilInput", func(t *testing.T) {
		err := token.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})
}
