//go:build testhooks

package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NativeInstanceRecords are the legacy instance row reads and writes the
// integration fixtures arrange their blocks with; the owner serves them
// over its instance rows.
type NativeInstanceRecords interface {
	Create(context.Context, *schedulerecords.ActivityInstance) error
	FindByID(context.Context, int64) (*schedulerecords.ActivityInstance, error)
	Update(context.Context, *schedulerecords.ActivityInstance) error
	Delete(context.Context, int64) error
	CreateTemplateBackedIfAbsent(ctx context.Context, instance *schedulerecords.ActivityInstance) (inserted bool, err error)
	FindByTenantAndDate(ctx context.Context, date calendar.Date) ([]*schedulerecords.ActivityInstance, error)
	FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*schedulerecords.ActivityInstance, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*schedulerecords.ActivityInstance, error)
	FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date calendar.Date) ([]*schedulerecords.ActivityInstance, error)
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to calendar.Date) ([]*schedulerecords.ActivityInstance, error)
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*schedulerecords.ActivityInstance, error)
	FindPlannedTemplateBackedFrom(ctx context.Context, from calendar.Date) ([]*schedulerecords.ActivityInstance, error)
	MaxID(ctx context.Context) (int64, error)
	MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
	DeletePlannedNonSpontaneousInWindow(ctx context.Context, from calendar.Date, to *calendar.Date, activityGroupID *int64, preserveDeviations bool) (int64, error)
	PropagateListKindToFutureInstances(ctx context.Context, activityGroupID int64, previousKind, newKind *string, after calendar.Date) (int64, error)
	UpdateColumns(ctx context.Context, instance *schedulerecords.ActivityInstance, columns ...string) (int64, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *calendar.Date) (*calendar.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff calendar.Date) (int64, error)
	ListInstances(ctx context.Context, filter LegacyInstanceFilter) ([]*schedulerecords.ActivityInstance, error)
}
