package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) FindStaffCalendarFeedOwner(ctx context.Context, tokenHash string) (identityaccess.StaffCalendarFeedOwner, bool, error) {
	owner, found, err := e.staffCalendarFeeds.FindStaffCalendarFeedOwner(ctx, tokenHash)
	return identityaccess.StaffCalendarFeedOwner(owner), found, mapError(err)
}
func (e engine) EnsureStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (string, error) {
	result, err := e.staffCalendarFeeds.EnsureStaffCalendarFeedToken(ctx, accountID, tenantID, tokenHash)
	return result, mapError(err)
}
func (e engine) RotateStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (bool, error) {
	result, err := e.staffCalendarFeeds.RotateStaffCalendarFeedToken(ctx, accountID, tenantID, tokenHash)
	return result, mapError(err)
}
