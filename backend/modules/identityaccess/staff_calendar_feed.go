package identityaccess

import "context"

// StaffCalendarFeedOwner identifies the active membership backing a feed token.
type StaffCalendarFeedOwner struct {
	AccountID int64
	TenantID  int64
}

// StaffCalendarFeeds persists hashed capability tokens on active memberships.
// The caller authorizes the explicit account and school; ambient transactions
// are preserved. Token generation, hashing and calendar content belong to Calendar.
type StaffCalendarFeeds interface {
	FindStaffCalendarFeedOwner(ctx context.Context, tokenHash string) (StaffCalendarFeedOwner, bool, error)
	EnsureStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (string, error)
	RotateStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (bool, error)
}
