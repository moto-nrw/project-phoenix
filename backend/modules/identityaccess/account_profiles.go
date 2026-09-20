package identityaccess

import "context"

// AccountProfiles reads school-scoped profile fields and changes only the bio.
// Both operations require a tenant and join the caller's transaction.
type AccountProfiles interface {
	FindAccountProfile(context.Context, int64) (bio, settings string, found bool, err error)
	SetAccountBio(context.Context, int64, string) error
}

func (m *Module) FindAccountProfile(ctx context.Context, accountID int64) (bio, settings string, found bool, err error) {
	return m.engine.FindAccountProfile(ctx, accountID)
}

func (m *Module) SetAccountBio(ctx context.Context, accountID int64, bio string) error {
	return m.engine.SetAccountBio(ctx, accountID, bio)
}
