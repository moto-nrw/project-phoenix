package application

import "context"

// Settings is the slice of the settings platform this package needs: the
// parent-news feature flag and the school logo for the e-mail header.
type Settings interface {
	ParentNewsEnabled(ctx context.Context) (bool, error)
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)
}
