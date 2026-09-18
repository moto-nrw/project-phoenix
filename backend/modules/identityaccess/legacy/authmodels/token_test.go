package authmodels

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestToken_MobileFlag(t *testing.T) {
	t.Parallel()

	t.Run("default is false", func(t *testing.T) {
		token := &Token{
			AccountID: 1,
			Token:     "token",
			Expiry:    time.Now().Add(time.Hour),
		}

		if token.Mobile != false {
			t.Error("Token.Mobile should default to false")
		}
	})

	t.Run("can be set to true", func(t *testing.T) {
		token := &Token{
			AccountID: 1,
			Token:     "token",
			Expiry:    time.Now().Add(time.Hour),
			Mobile:    true,
		}

		if token.Mobile != true {
			t.Error("Token.Mobile should be true when set")
		}
	})
}

func TestToken_FamilyTracking(t *testing.T) {
	t.Parallel()

	t.Run("token family fields", func(t *testing.T) {
		token := &Token{
			AccountID:  1,
			Token:      "token",
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   "family-123",
			Generation: 3,
		}

		if token.FamilyID != "family-123" {
			t.Errorf("Token.FamilyID = %q, want %q", token.FamilyID, "family-123")
		}

		if token.Generation != 3 {
			t.Errorf("Token.Generation = %d, want %d", token.Generation, 3)
		}
	})

	t.Run("default generation is 0", func(t *testing.T) {
		token := &Token{
			AccountID: 1,
			Token:     "token",
			Expiry:    time.Now().Add(time.Hour),
		}

		if token.Generation != 0 {
			t.Errorf("Token.Generation should default to 0, got %d", token.Generation)
		}
	})
}

func TestToken_GetID(t *testing.T) {
	t.Parallel()

	token := &Token{
		Model:     base.Model{ID: 42},
		AccountID: 1,
		Token:     "test",
		Expiry:    time.Now().Add(time.Hour),
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := token.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", token.GetID())
	}
}

func TestToken_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &Token{
		Model:     base.Model{CreatedAt: now},
		AccountID: 1,
		Token:     "test",
		Expiry:    time.Now().Add(time.Hour),
	}

	if got := token.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestToken_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &Token{
		Model:     base.Model{UpdatedAt: now},
		AccountID: 1,
		Token:     "test",
		Expiry:    time.Now().Add(time.Hour),
	}

	if got := token.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
