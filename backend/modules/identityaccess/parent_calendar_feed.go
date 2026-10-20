package identityaccess

import "context"

// ParentCalendarFeedAccount is the account fact needed to create and resolve
// parent feed credentials. Calendar still enforces activation and child access.
type ParentCalendarFeedAccount struct {
	ID        int64
	Email     string
	Active    bool
	TokenHash string
}

// ParentCalendarFeeds stores hashes, never raw feed credentials. The caller
// authorizes account IDs; these account-wide operations preserve ambient transactions.
type ParentCalendarFeeds interface {
	FindParentCalendarFeedAccount(context.Context, int64) (ParentCalendarFeedAccount, bool, error)
	FindParentCalendarFeedOwner(context.Context, string) (ParentCalendarFeedAccount, bool, error)
	EnsureParentCalendarFeedToken(context.Context, int64, string) (string, error)
	RotateParentCalendarFeedToken(context.Context, int64, string) error
}
