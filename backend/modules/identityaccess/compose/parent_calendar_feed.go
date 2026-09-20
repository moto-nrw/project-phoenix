package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) FindParentCalendarFeedAccount(ctx context.Context, id int64) (identityaccess.ParentCalendarFeedAccount, bool, error) {
	value, found, err := e.parentCalendarFeeds.FindParentCalendarFeedAccount(ctx, id)
	return identityaccess.ParentCalendarFeedAccount(value), found, mapError(err)
}
func (e engine) FindParentCalendarFeedOwner(ctx context.Context, hash string) (identityaccess.ParentCalendarFeedAccount, bool, error) {
	value, found, err := e.parentCalendarFeeds.FindParentCalendarFeedOwner(ctx, hash)
	return identityaccess.ParentCalendarFeedAccount(value), found, mapError(err)
}
func (e engine) EnsureParentCalendarFeedToken(ctx context.Context, id int64, hash string) (string, error) {
	value, err := e.parentCalendarFeeds.EnsureParentCalendarFeedToken(ctx, id, hash)
	return value, mapError(err)
}
func (e engine) RotateParentCalendarFeedToken(ctx context.Context, id int64, hash string) error {
	return mapError(e.parentCalendarFeeds.RotateParentCalendarFeedToken(ctx, id, hash))
}
