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

func (m *Module) FindParentCalendarFeedAccount(ctx context.Context, id int64) (ParentCalendarFeedAccount, bool, error) {
	return m.engine.FindParentCalendarFeedAccount(ctx, id)
}
func (m *Module) FindParentCalendarFeedOwner(ctx context.Context, hash string) (ParentCalendarFeedAccount, bool, error) {
	return m.engine.FindParentCalendarFeedOwner(ctx, hash)
}
func (m *Module) EnsureParentCalendarFeedToken(ctx context.Context, id int64, hash string) (string, error) {
	return m.engine.EnsureParentCalendarFeedToken(ctx, id, hash)
}
func (m *Module) RotateParentCalendarFeedToken(ctx context.Context, id int64, hash string) error {
	return m.engine.RotateParentCalendarFeedToken(ctx, id, hash)
}
