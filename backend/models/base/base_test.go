package base

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestModel_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	m := Model{
		ID:        42,
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	}

	if m.ID != 42 {
		t.Errorf("Model.ID = %v, want 42", m.ID)
	}

	if !m.CreatedAt.Equal(now) {
		t.Errorf("Model.CreatedAt = %v, want %v", m.CreatedAt, now)
	}

	if !m.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Errorf("Model.UpdatedAt = %v, want %v", m.UpdatedAt, now.Add(time.Hour))
	}
}

func TestStringIDModel_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	m := StringIDModel{
		ID:        "RFID12345678",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	}

	if m.ID != "RFID12345678" {
		t.Errorf("StringIDModel.ID = %v, want RFID12345678", m.ID)
	}

	if !m.CreatedAt.Equal(now) {
		t.Errorf("StringIDModel.CreatedAt = %v, want %v", m.CreatedAt, now)
	}

	if !m.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Errorf("StringIDModel.UpdatedAt = %v, want %v", m.UpdatedAt, now.Add(time.Hour))
	}
}

func TestDatabaseError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *DatabaseError
		expected string
	}{
		{
			name: "with original error",
			err: &DatabaseError{
				Op:  "create",
				Err: errors.New("connection refused"),
			},
			expected: "database error during create: connection refused",
		},
		{
			name: "without original error",
			err: &DatabaseError{
				Op:  "update",
				Err: nil,
			},
			expected: "database error during update",
		},
		{
			name: "with delete operation",
			err: &DatabaseError{
				Op:  "delete",
				Err: errors.New("foreign key constraint"),
			},
			expected: "database error during delete: foreign key constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("DatabaseError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDatabaseError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("with original error", func(t *testing.T) {
		originalErr := errors.New("original error")
		dbErr := &DatabaseError{
			Op:  "create",
			Err: originalErr,
		}

		if got := dbErr.Unwrap(); got != originalErr {
			t.Errorf("DatabaseError.Unwrap() = %v, want %v", got, originalErr)
		}
	})

	t.Run("without original error", func(t *testing.T) {
		dbErr := &DatabaseError{
			Op:  "create",
			Err: nil,
		}

		if got := dbErr.Unwrap(); got != nil {
			t.Errorf("DatabaseError.Unwrap() = %v, want nil", got)
		}
	})
}

func TestContextWithTx_And_TxFromContext(t *testing.T) {
	t.Parallel()

	t.Run("extract tx from context without tx", func(t *testing.T) {
		ctx := context.Background()

		tx, ok := TxFromContext(ctx)
		if ok {
			t.Error("TxFromContext() should return false when no tx in context")
		}
		if tx != nil {
			t.Errorf("TxFromContext() = %v, want nil", tx)
		}
	})

	// Note: Testing with a real bun.Tx requires a database connection
	// The functions ContextWithTx and TxFromContext are tested together
	// in integration tests that have database access
}
