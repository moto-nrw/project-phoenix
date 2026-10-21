//go:build testhooks

package fixturetest

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// InstanceReadsOf serves a fake instance repository as the joined instance
// read: each filter shape the owner's reads use goes to the fake's matching
// legacy read.
func InstanceReadsOf(fake any) ports.LegacyInstanceReads {
	return fakeInstanceReads{fake: fake}
}

// InstanceRowsOf serves a fake instance repository as the owner's instance
// rows: reads as InstanceReadsOf, every plan patch and a discarded session
// as the fake's column update. A suite that starts or completes a session
// sets its own Sessions.
func InstanceRowsOf(fake ActivityInstanceRecords) ports.InstanceRows {
	return ports.InstanceRows{
		Plans:    fakeInstancePlans{fake: fake},
		Sessions: fakeInstanceSessions{fake: fake},
		Reads:    fakeInstanceReads{fake: fake},
	}
}

// fakeInstanceReads asks the fake for the legacy read the filter matches;
// a fake that lacks it fails the read.
type fakeInstanceReads struct {
	fake any
}

func (r fakeInstanceReads) ListLegacyInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*records.ActivityInstance, error) {
	byID, _ := r.fake.(interface {
		FindByID(context.Context, int64) (*ActivityInstance, error)
	})
	byIDs, _ := r.fake.(interface {
		FindByIDs(context.Context, []int64) ([]*ActivityInstance, error)
	})
	bySession, _ := r.fake.(interface {
		FindByActiveGroupID(context.Context, int64) (*ActivityInstance, error)
	})
	byDate, _ := r.fake.(interface {
		FindByTenantAndDate(context.Context, calendar.Date) ([]*ActivityInstance, error)
	})
	byRange, _ := r.fake.(interface {
		FindByTenantAndDateRange(context.Context, calendar.Date, calendar.Date) ([]*ActivityInstance, error)
	})
	switch {
	case len(filter.IDs) == 1 && byID != nil:
		row, err := byID.FindByID(ctx, filter.IDs[0])
		return single(row), err
	case len(filter.IDs) > 1 && byIDs != nil:
		rows, err := byIDs.FindByIDs(ctx, filter.IDs)
		return toNative3(rows), err
	case len(filter.ActiveGroupIDs) == 1 && filter.Limit == 1 && bySession != nil:
		row, err := bySession.FindByActiveGroupID(ctx, filter.ActiveGroupIDs[0])
		return single(row), err
	case filter.Date != nil && filter.ActivityGroupID == nil && byDate != nil:
		rows, err := byDate.FindByTenantAndDate(ctx, calendar.Date(*filter.Date))
		return toNative3(rows), err
	case filter.FromDate != nil && filter.ToDate != nil && filter.ActivityGroupID == nil && byRange != nil:
		rows, err := byRange.FindByTenantAndDateRange(ctx, calendar.Date(*filter.FromDate), calendar.Date(*filter.ToDate))
		return toNative3(rows), err
	}
	if listing, ok := r.fake.(InstanceListing); ok {
		rows, err := listing.ListInstances(ctx, filter)
		return toNative3(rows), err
	}
	return nil, fmt.Errorf("fixture instance read is not configured for %+v", filter)
}

func single(row *ActivityInstance) []*records.ActivityInstance {
	if row == nil {
		return []*records.ActivityInstance{}
	}
	return []*records.ActivityInstance{toNative2(row)}
}

type fakeInstancePlans struct {
	timetable.ActivityInstanceCapability
	fake ActivityInstanceRecords
}

func (p fakeInstancePlans) PatchActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput, columns []string) (int64, error) {
	row := NewActivityInstance(func(row *ActivityInstance) { row.Status = input.Status })
	row.ID = id
	return p.fake.UpdateColumns(ctx, row, columns...)
}

type fakeInstanceSessions struct {
	ports.InstanceSessions
	fake ActivityInstanceRecords
}

func (s fakeInstanceSessions) DiscardActivitySession(ctx context.Context, instanceID int64) error {
	return s.transition(ctx, instanceID, records.InstanceStatusCancelled)
}

func (s fakeInstanceSessions) transition(ctx context.Context, instanceID int64, status string) error {
	row := NewActivityInstance(func(row *ActivityInstance) { row.Status = status })
	row.ID = instanceID
	_, err := s.fake.UpdateColumns(ctx, row, "status")
	return err
}
