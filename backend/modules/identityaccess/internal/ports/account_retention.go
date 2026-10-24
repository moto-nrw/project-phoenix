package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// AccountRetentionStore tombstones the accounts no school needs any more
// (#4043) through the database workflow auth.erase_unused_accounts (ADR
// 0065). It runs in the token cleanup's administrative transaction and
// returns each erased account with the avatar files it held.
type AccountRetentionStore interface {
	EraseUnusedAccounts(ctx context.Context) ([]domain.ErasedAccount, domain.OperationStats, error)
	PendingAvatarCleanups(ctx context.Context) ([]domain.AvatarCleanup, error)
	FinishAvatarCleanup(ctx context.Context, id int64, removed bool) error
}
