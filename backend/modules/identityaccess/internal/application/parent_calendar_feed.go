package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type ParentCalendarFeeds struct {
	service *Service
	store   ports.ParentCalendarFeedStore
}

func NewParentCalendarFeeds(service *Service, store ports.ParentCalendarFeedStore) *ParentCalendarFeeds {
	return &ParentCalendarFeeds{service: service, store: store}
}
func (f *ParentCalendarFeeds) FindParentCalendarFeedAccount(ctx context.Context, id int64) (identityaccess.ParentCalendarFeedAccount, bool, error) {
	var result domain.ParentCalendarFeedAccount
	var found bool
	err := f.service.run(ctx, f.service.tx.RunPlatform, "find_parent_calendar_feed_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, found, queryStats, queryErr = f.store.FindParentCalendarFeedAccount(txCtx, id)
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return identityaccess.ParentCalendarFeedAccount(result), found, MapError(err)
	}
	return identityaccess.ParentCalendarFeedAccount(result), found, nil
}
func (f *ParentCalendarFeeds) FindParentCalendarFeedOwner(ctx context.Context, hash string) (identityaccess.ParentCalendarFeedAccount, bool, error) {
	var result domain.ParentCalendarFeedAccount
	var found bool
	err := f.service.run(ctx, f.service.tx.RunPlatform, "find_parent_calendar_feed_owner", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, found, queryStats, queryErr = f.store.FindParentCalendarFeedOwner(txCtx, hash)
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return identityaccess.ParentCalendarFeedAccount(result), found, MapError(err)
	}
	return identityaccess.ParentCalendarFeedAccount(result), found, nil
}
func (f *ParentCalendarFeeds) EnsureParentCalendarFeedToken(ctx context.Context, id int64, hash string) (result string, err error) {
	defer func() { err = MapError(err) }()
	err = f.service.run(ctx, f.service.tx.RunPlatform, "ensure_parent_calendar_feed_token", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = f.store.EnsureParentCalendarFeedToken(txCtx, id, hash)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}
func (f *ParentCalendarFeeds) RotateParentCalendarFeedToken(ctx context.Context, id int64, hash string) (err error) {
	defer func() { err = MapError(err) }()
	err = f.service.run(ctx, f.service.tx.RunPlatform, "rotate_parent_calendar_feed_token", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		queryStats, queryErr = f.store.RotateParentCalendarFeedToken(txCtx, id, hash)
		stats.Add(queryStats)
		return queryErr
	})
	return err
}
