package identityaccess

import (
	"context"
)

// AccountRetention tombstones the accounts no school needs any more (#4043):
// no pending school access, no active one with a role other than guardian,
// no person, parent contact, enrollment request, open invitation or demo
// access. The account's own data goes; the row stays with a placeholder
// address, because messages and protocols keep pointing at it. The hourly
// token cleanup runs it in an administrative transaction that reaches every
// school.
type AccountRetention interface {
	EraseUnusedAccounts(ctx context.Context) (int, error)
	CleanupErasedAccountAvatars(ctx context.Context) (int, error)
}
