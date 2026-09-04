package enrollment

import (
	"context"
	"errors"
)

// DirectoryGuardian is the People Directory projection the enrollment
// deletion preview reads: users.guardian_profiles belongs to that owner
// (#2663), so the profiles of a request's account and their link counts are
// resolved through it instead of a foreign join.
type DirectoryGuardian struct {
	ID        int64
	AccountID *int64
}

// GuardianDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type GuardianDirectory interface {
	// ListGuardiansByAccount returns the tenant's profiles linked to the
	// accounts.
	ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]DirectoryGuardian, error)
	// ListGuardiansByID returns the tenant's profiles for the ids.
	ListGuardiansByID(ctx context.Context, ids []int64) ([]DirectoryGuardian, error)
	// CountGuardianLinks counts the tenant's links per profile; profiles
	// without a link are absent.
	CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error)
}

var errGuardianDirectoryRequired = errors.New("enrollment repositories: guardian directory is not bound")
