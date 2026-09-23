package test

import (
	"context"

	"github.com/uptrace/bun"
)

// AccountState is persisted evidence for authentication behavior assertions.
// It is not an account domain model or a serving persistence capability.
type AccountState struct {
	Active       bool
	PasswordHash *string
	Avatar       string
}

// ReadAccountState reads only the credential and profile facts tests assert.
func ReadAccountState(ctx context.Context, db bun.IDB, accountID int64) (AccountState, error) {
	var state AccountState
	err := db.NewRaw("SELECT active, password_hash, avatar FROM auth.accounts WHERE id = ?", accountID).Scan(ctx, &state)
	return state, err
}

// AccountEmailExists verifies creation or rollback independently of the owner.
func AccountEmailExists(ctx context.Context, db bun.IDB, email string) (bool, error) {
	var exists bool
	err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM auth.accounts WHERE LOWER(email) = LOWER(?))", email).Scan(ctx, &exists)
	return exists, err
}
