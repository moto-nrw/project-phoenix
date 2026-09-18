package authmodels

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestPasswordResetToken_GetID(t *testing.T) {
	t.Parallel()

	token := &PasswordResetToken{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := token.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", token.GetID())
	}
}

func TestPasswordResetToken_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &PasswordResetToken{
		Model: base.Model{CreatedAt: now},
	}

	if got := token.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestPasswordResetToken_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &PasswordResetToken{
		Model: base.Model{UpdatedAt: now},
	}

	if got := token.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
