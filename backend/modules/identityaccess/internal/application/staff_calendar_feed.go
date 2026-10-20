package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type StaffCalendarFeeds struct {
	service *Service
	store   ports.StaffCalendarFeedStore
}

func NewStaffCalendarFeeds(service *Service, store ports.StaffCalendarFeedStore) *StaffCalendarFeeds {
	return &StaffCalendarFeeds{service: service, store: store}
}
func (f *StaffCalendarFeeds) FindStaffCalendarFeedOwner(ctx context.Context, tokenHash string) (identityaccess.StaffCalendarFeedOwner, bool, error) {
	var result domain.StaffCalendarFeedOwner
	var found bool
	err := f.service.run(ctx, f.service.tx.RunPlatform, "find_staff_calendar_feed_owner", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, found, queryStats, queryErr = f.store.FindStaffCalendarFeedOwner(txCtx, tokenHash)
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return identityaccess.StaffCalendarFeedOwner(result), found, MapError(err)
	}
	return identityaccess.StaffCalendarFeedOwner(result), found, nil
}
func (f *StaffCalendarFeeds) EnsureStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (result string, err error) {
	defer func() { err = MapError(err) }()
	err = f.service.run(ctx, f.service.tx.RunPlatform, "ensure_staff_calendar_feed_token", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = f.store.EnsureStaffCalendarFeedToken(txCtx, accountID, tenantID, tokenHash)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}
func (f *StaffCalendarFeeds) RotateStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (result bool, err error) {
	defer func() { err = MapError(err) }()
	err = f.service.run(ctx, f.service.tx.RunPlatform, "rotate_staff_calendar_feed_token", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = f.store.RotateStaffCalendarFeedToken(txCtx, accountID, tenantID, tokenHash)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}
