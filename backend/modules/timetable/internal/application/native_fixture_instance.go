//go:build testhooks

package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// nativeFixtureInstances serve the integration fixtures' legacy instance
// row reads and writes over the owner's instance rows.
type nativeFixtureInstances struct {
	rows ports.InstanceRows
}

// NativeInstanceRecords binds the fixtures' instance rows.
func NativeInstanceRecords(rows ports.InstanceRows) ports.NativeInstanceRecords {
	return nativeFixtureInstances{rows: rows}
}

func (f nativeFixtureInstances) Create(ctx context.Context, value *scheduleModels.ActivityInstance) error {
	return createLegacyInstance(ctx, f.rows, value)
}

func (f nativeFixtureInstances) FindByID(ctx context.Context, id int64) (*scheduleModels.ActivityInstance, error) {
	return findLegacyInstance(ctx, f.rows.Reads, id)
}

func (f nativeFixtureInstances) Update(ctx context.Context, value *scheduleModels.ActivityInstance) error {
	return updateLegacyInstance(ctx, f.rows, value)
}

func (f nativeFixtureInstances) Delete(ctx context.Context, id int64) error {
	return deleteLegacyInstance(ctx, f.rows.Plans, id)
}

func (f nativeFixtureInstances) CreateTemplateBackedIfAbsent(ctx context.Context, value *scheduleModels.ActivityInstance) (bool, error) {
	return createTemplateBackedInstanceIfAbsent(ctx, f.rows, value)
}

func (f nativeFixtureInstances) FindByTenantAndDate(ctx context.Context, date calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	return instancesOn(ctx, f.rows.Reads, date)
}

func (f nativeFixtureInstances) FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	return instancesBetween(ctx, f.rows.Reads, from, to)
}

func (f nativeFixtureInstances) FindByIDs(ctx context.Context, ids []int64) ([]*scheduleModels.ActivityInstance, error) {
	return instancesByID(ctx, f.rows.Reads, ids)
}

func (f nativeFixtureInstances) FindByActivityGroupAndDate(ctx context.Context, groupID int64, date calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	return groupInstancesOn(ctx, f.rows.Reads, groupID, date)
}

func (f nativeFixtureInstances) FindByActivityGroupAndDateRange(ctx context.Context, groupID int64, from, to calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	return groupInstancesBetween(ctx, f.rows.Reads, groupID, from, to)
}

func (f nativeFixtureInstances) FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModels.ActivityInstance, error) {
	return sessionInstance(ctx, f.rows.Reads, activeGroupID)
}

func (f nativeFixtureInstances) FindPlannedTemplateBackedFrom(ctx context.Context, from calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	return plannedTemplateBackedInstancesFrom(ctx, f.rows.Reads, from)
}

func (f nativeFixtureInstances) MaxID(ctx context.Context) (int64, error) {
	return maxLegacyInstanceID(ctx, f.rows.Plans)
}

func (f nativeFixtureInstances) MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	return markInstanceCompleted(ctx, f.rows.Sessions, instanceID, completedAt)
}

func (f nativeFixtureInstances) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	return completeSessionInstances(ctx, f.rows.Sessions, activeGroupIDs, completedAt)
}

func (f nativeFixtureInstances) DeletePlannedNonSpontaneousInWindow(ctx context.Context, from calendar.Date, to *calendar.Date, groupID *int64, preserveDeviations bool) (int64, error) {
	return deletePlannedInstancesInWindow(ctx, f.rows.Plans, from, to, groupID, preserveDeviations)
}

func (f nativeFixtureInstances) PropagateListKindToFutureInstances(ctx context.Context, groupID int64, previousKind, newKind *string, after calendar.Date) (int64, error) {
	return propagateInstanceListKind(ctx, f.rows.Plans, groupID, previousKind, newKind, after)
}

func (f nativeFixtureInstances) UpdateColumns(ctx context.Context, value *scheduleModels.ActivityInstance, columns ...string) (int64, error) {
	return updateLegacyInstanceColumns(ctx, f.rows, value, columns...)
}

func (f nativeFixtureInstances) OldestBefore(ctx context.Context, column string, cutoff *calendar.Date) (*calendar.Date, error) {
	return oldestInstanceBefore(ctx, f.rows.Plans, column, cutoff)
}

func (f nativeFixtureInstances) DeleteOlderThan(ctx context.Context, column string, cutoff calendar.Date) (int64, error) {
	return deleteInstancesOlderThan(ctx, f.rows.Plans, column, cutoff)
}

func (f nativeFixtureInstances) ListInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	return listLegacyInstances(ctx, f.rows.Reads, filter, "list with options")
}

// The fixtures alone still update, number, stamp and expire instances
// through the legacy rows.

// updateLegacyInstance replaces the planning fields and reloads the row.
func updateLegacyInstance(ctx context.Context, rows ports.InstanceRows, value *scheduleModels.ActivityInstance) error {
	if value == nil {
		return errors.New("activity instance cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := rows.Plans.UpdateActivityInstance(ctx, value.ID, publicActivityInstanceInput(value))
	if err != nil {
		return legacyActivityInstanceError("update", err)
	}
	return reloadLegacyInstance(ctx, rows.Reads, value, updated.ID, "update")
}

func maxLegacyInstanceID(ctx context.Context, plans timetable.ActivityInstanceQuery) (int64, error) {
	value, err := plans.MaxActivityInstanceID(ctx)
	if err != nil {
		return 0, WrapDatabaseError("get max activity instance id", err)
	}
	return value, nil
}

// markInstanceCompleted stamps the instance completed at the instant, as
// the nightly close always did.
func markInstanceCompleted(ctx context.Context, sessions ports.InstanceSessions, instanceID int64, completedAt time.Time) error {
	if err := sessions.RecordActivitySessionCompleted(ctx, instanceID, completedAt); err != nil {
		if errors.Is(err, studentpresence.ErrActivitySessionNotFound) {
			return WrapNotFoundDatabaseError("mark completed")
		}
		return WrapDatabaseError("mark completed", err)
	}
	return nil
}

// oldestInstanceBefore reads the oldest instance date before the cutoff;
// only the date column is supported.
func oldestInstanceBefore(ctx context.Context, plans timetable.ActivityInstanceQuery, column string, cutoff *calendar.Date) (*calendar.Date, error) {
	if column != legacyActivityInstanceDateColumn {
		return nil, WrapDatabaseError("oldest before", fmt.Errorf("unsupported activity instance date column %q", column))
	}
	value, err := plans.OldestActivityInstanceBefore(ctx, publicOptionalInstanceDate(cutoff))
	if err != nil {
		return nil, WrapDatabaseError("oldest before", err)
	}
	if value == nil {
		return nil, nil
	}
	result, err := ParseActivityInstanceDate(*value)
	if err != nil {
		return nil, WrapDatabaseError("oldest before", err)
	}
	return &result, nil
}

// deleteInstancesOlderThan removes the instances dated before the cutoff;
// only the date column is supported.
func deleteInstancesOlderThan(ctx context.Context, plans timetable.ActivityInstanceCommand, column string, cutoff calendar.Date) (int64, error) {
	if column != legacyActivityInstanceDateColumn {
		return 0, WrapDatabaseError("delete older than", fmt.Errorf("unsupported activity instance date column %q", column))
	}
	rows, err := plans.DeleteActivityInstancesBefore(ctx, cutoff.String())
	if err != nil {
		return 0, WrapDatabaseError("delete older than", err)
	}
	return rows, nil
}

func legacyActivityInstanceError(operation string, err error) error {
	if errors.Is(err, timetable.ErrActivityInstanceNotFound) {
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}

const legacyActivityInstanceDateColumn = "date"
