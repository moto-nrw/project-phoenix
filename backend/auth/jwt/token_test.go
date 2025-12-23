package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestToken_BeforeInsert(t *testing.T) {
	tests := []struct {
		name      string
		token     Token
		checkFunc func(t *testing.T, token *Token, originalCreatedAt time.Time)
	}{
		{
			name: "sets timestamps when zero",
			token: Token{
				ID:        1,
				AccountID: 123,
				Token:     "test-token",
				Expiry:    time.Now().Add(24 * time.Hour),
			},
			checkFunc: func(t *testing.T, token *Token, originalCreatedAt time.Time) {
				assert.False(t, token.CreatedAt.IsZero(), "CreatedAt should be set")
				assert.False(t, token.UpdatedAt.IsZero(), "UpdatedAt should be set")
				assert.Equal(t, token.CreatedAt, token.UpdatedAt, "CreatedAt and UpdatedAt should be equal on insert")
			},
		},
		{
			name: "preserves existing timestamps",
			token: Token{
				ID:        1,
				AccountID: 123,
				Token:     "test-token",
				Expiry:    time.Now().Add(24 * time.Hour),
				CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			checkFunc: func(t *testing.T, token *Token, originalCreatedAt time.Time) {
				// When CreatedAt is already set, BeforeInsert should not modify it
				assert.Equal(t, originalCreatedAt, token.CreatedAt, "CreatedAt should not change")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCreatedAt := tt.token.CreatedAt

			// BeforeInsert expects *bun.DB but doesn't use it, so we can pass nil
			var db *bun.DB
			err := tt.token.BeforeInsert(db)

			require.NoError(t, err)
			tt.checkFunc(t, &tt.token, originalCreatedAt)
		})
	}
}

func TestToken_BeforeUpdate(t *testing.T) {
	tests := []struct {
		name  string
		token Token
	}{
		{
			name: "updates UpdatedAt timestamp",
			token: Token{
				ID:        1,
				AccountID: 123,
				Token:     "test-token",
				Expiry:    time.Now().Add(24 * time.Hour),
				CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "works with zero UpdatedAt",
			token: Token{
				ID:        2,
				AccountID: 456,
				Token:     "another-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalUpdatedAt := tt.token.UpdatedAt
			beforeUpdate := time.Now()

			var db *bun.DB
			err := tt.token.BeforeUpdate(db)

			afterUpdate := time.Now()

			require.NoError(t, err)

			// UpdatedAt should be between beforeUpdate and afterUpdate
			assert.True(t, tt.token.UpdatedAt.After(beforeUpdate) || tt.token.UpdatedAt.Equal(beforeUpdate))
			assert.True(t, tt.token.UpdatedAt.Before(afterUpdate) || tt.token.UpdatedAt.Equal(afterUpdate))

			// UpdatedAt should have changed (unless it was already very recent)
			if !originalUpdatedAt.IsZero() && time.Since(originalUpdatedAt) > time.Millisecond {
				assert.NotEqual(t, originalUpdatedAt, tt.token.UpdatedAt)
			}
		})
	}
}

func TestToken_Claims(t *testing.T) {
	tests := []struct {
		name     string
		token    Token
		expected RefreshClaims
	}{
		{
			name: "returns correct refresh claims",
			token: Token{
				ID:        42,
				Token:     "unique-token-string",
				AccountID: 123,
			},
			expected: RefreshClaims{
				ID:    42,
				Token: "unique-token-string",
			},
		},
		{
			name: "handles zero ID",
			token: Token{
				ID:    0,
				Token: "zero-id-token",
			},
			expected: RefreshClaims{
				ID:    0,
				Token: "zero-id-token",
			},
		},
		{
			name: "handles empty token string",
			token: Token{
				ID:    1,
				Token: "",
			},
			expected: RefreshClaims{
				ID:    1,
				Token: "",
			},
		},
		{
			name: "handles special characters in token",
			token: Token{
				ID:    999,
				Token: "token-with-special!@#$%^&*()",
			},
			expected: RefreshClaims{
				ID:    999,
				Token: "token-with-special!@#$%^&*()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := tt.token.Claims()

			assert.Equal(t, tt.expected.ID, claims.ID)
			assert.Equal(t, tt.expected.Token, claims.Token)
		})
	}
}

func TestToken_FullLifecycle(t *testing.T) {
	// Test the full lifecycle of a Token struct
	token := Token{
		AccountID:  100,
		Token:      "lifecycle-test-token",
		Expiry:     time.Now().Add(24 * time.Hour),
		Mobile:     true,
		Identifier: "device-123",
	}

	// 1. Before insert sets timestamps
	var db *bun.DB
	err := token.BeforeInsert(db)
	require.NoError(t, err)
	assert.False(t, token.CreatedAt.IsZero())
	assert.False(t, token.UpdatedAt.IsZero())
	createdAt := token.CreatedAt

	// Simulate time passing
	time.Sleep(1 * time.Millisecond)

	// 2. Before update changes UpdatedAt
	err = token.BeforeUpdate(db)
	require.NoError(t, err)
	assert.True(t, token.UpdatedAt.After(createdAt) || token.UpdatedAt.Equal(createdAt))
	assert.Equal(t, createdAt, token.CreatedAt, "CreatedAt should not change")

	// 3. Claims returns correct data
	claims := token.Claims()
	assert.Equal(t, token.ID, claims.ID)
	assert.Equal(t, token.Token, claims.Token)
}

func TestToken_Fields(t *testing.T) {
	// Test that all fields are properly set
	expiry := time.Now().Add(48 * time.Hour)
	token := Token{
		ID:         42,
		AccountID:  100,
		Token:      "my-token-value",
		Expiry:     expiry,
		Mobile:     true,
		Identifier: "mobile-device-uuid",
	}

	assert.Equal(t, 42, token.ID)
	assert.Equal(t, 100, token.AccountID)
	assert.Equal(t, "my-token-value", token.Token)
	assert.Equal(t, expiry, token.Expiry)
	assert.True(t, token.Mobile)
	assert.Equal(t, "mobile-device-uuid", token.Identifier)
}
