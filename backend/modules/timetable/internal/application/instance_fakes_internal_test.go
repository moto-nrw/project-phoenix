package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// fakeInstanceRows serves a fake written against the retained instance
// repository as the owner's instance rows: every read, plan write and
// session transition goes to the fake's matching legacy method. A method the
// fake lacks fails the call.
func fakeInstanceRows(fake any) ports.InstanceRows {
	created := map[int64]*scheduleModels.ActivityInstance{}
	return ports.InstanceRows{
		Plans:    legacyInstancePlans{fake: fake, created: created},
		Sessions: legacyInstanceSessions{fake: fake},
		Reads:    legacyInstanceReads{fake: fake, created: created},
	}
}

// legacyInstanceReads answers the reload after a create with the row the
// fake was handed, and every other read from the fake.
type legacyInstanceReads struct {
	fake    any
	created map[int64]*scheduleModels.ActivityInstance
}

func (r legacyInstanceReads) ListLegacyInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	if len(filter.IDs) == 1 {
		if row, ok := r.created[filter.IDs[0]]; ok {
			return []*scheduleModels.ActivityInstance{row}, nil
		}
	}
	if fake, ok := r.fake.(interface {
		ListInstances(context.Context, ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error)
	}); ok && !singleInstanceRead(filter) {
		return fake.ListInstances(ctx, filter)
	}
	switch {
	case len(filter.IDs) == 1:
		if fake, ok := r.fake.(interface {
			FindByID(context.Context, int64) (*scheduleModels.ActivityInstance, error)
		}); ok {
			row, err := fake.FindByID(ctx, filter.IDs[0])
			return oneInstance(row), err
		}
	case len(filter.IDs) > 1:
		if fake, ok := r.fake.(interface {
			FindByIDs(context.Context, []int64) ([]*scheduleModels.ActivityInstance, error)
		}); ok {
			return fake.FindByIDs(ctx, filter.IDs)
		}
	case len(filter.ActiveGroupIDs) == 1 && filter.Limit == 1:
		if fake, ok := r.fake.(interface {
			FindByActiveGroupID(context.Context, int64) (*scheduleModels.ActivityInstance, error)
		}); ok {
			row, err := fake.FindByActiveGroupID(ctx, filter.ActiveGroupIDs[0])
			return oneInstance(row), err
		}
	case filter.MaterializedPlanned && filter.FromDate != nil:
		if fake, ok := r.fake.(interface {
			FindPlannedTemplateBackedFrom(context.Context, calendar.Date) ([]*scheduleModels.ActivityInstance, error)
		}); ok {
			return fake.FindPlannedTemplateBackedFrom(ctx, calendar.Date(*filter.FromDate))
		}
	case filter.ActivityGroupID != nil && filter.Date != nil:
		if fake, ok := r.fake.(interface {
			FindByActivityGroupAndDate(context.Context, int64, calendar.Date) ([]*scheduleModels.ActivityInstance, error)
		}); ok {
			return fake.FindByActivityGroupAndDate(ctx, *filter.ActivityGroupID, calendar.Date(*filter.Date))
		}
	case filter.ActivityGroupID != nil && filter.FromDate != nil && filter.ToDate != nil:
		if fake, ok := r.fake.(interface {
			FindByActivityGroupAndDateRange(context.Context, int64, calendar.Date, calendar.Date) ([]*scheduleModels.ActivityInstance, error)
		}); ok {
			return fake.FindByActivityGroupAndDateRange(ctx, *filter.ActivityGroupID, calendar.Date(*filter.FromDate), calendar.Date(*filter.ToDate))
		}
	case filter.Date != nil:
		if fake, ok := r.fake.(interface {
			FindByTenantAndDate(context.Context, calendar.Date) ([]*scheduleModels.ActivityInstance, error)
		}); ok {
			return fake.FindByTenantAndDate(ctx, calendar.Date(*filter.Date))
		}
	case filter.FromDate != nil && filter.ToDate != nil:
		if fake, ok := r.fake.(interface {
			FindByTenantAndDateRange(context.Context, calendar.Date, calendar.Date) ([]*scheduleModels.ActivityInstance, error)
		}); ok {
			return fake.FindByTenantAndDateRange(ctx, calendar.Date(*filter.FromDate), calendar.Date(*filter.ToDate))
		}
	}
	if fake, ok := r.fake.(interface {
		ListInstances(context.Context, ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error)
	}); ok {
		return fake.ListInstances(ctx, filter)
	}
	return nil, fmt.Errorf("fake %T serves no read for %+v", r.fake, filter)
}

// singleInstanceRead is a read of one instance, which a fake's FindByID or
// FindByActiveGroupID answers before its listing.
func singleInstanceRead(filter ports.LegacyInstanceFilter) bool {
	return len(filter.IDs) == 1 || (len(filter.ActiveGroupIDs) == 1 && filter.Limit == 1)
}

func oneInstance(row *scheduleModels.ActivityInstance) []*scheduleModels.ActivityInstance {
	if row == nil {
		return []*scheduleModels.ActivityInstance{}
	}
	return []*scheduleModels.ActivityInstance{row}
}

type legacyInstancePlans struct {
	timetable.ActivityInstanceCapability
	fake    any
	created map[int64]*scheduleModels.ActivityInstance
}

func (p legacyInstancePlans) CreateActivityInstance(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	fake, ok := p.fake.(interface {
		Create(context.Context, *scheduleModels.ActivityInstance) error
	})
	if !ok {
		return timetable.ActivityInstance{}, fmt.Errorf("fake %T creates no instance", p.fake)
	}
	row := legacyInstanceOfInput(input)
	err := fake.Create(ctx, row)
	p.created[row.ID] = row
	return timetable.ActivityInstance{ID: row.ID}, err
}

func (p legacyInstancePlans) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	fake, ok := p.fake.(interface {
		CreateTemplateBackedIfAbsent(context.Context, *scheduleModels.ActivityInstance) (bool, error)
	})
	if !ok {
		return timetable.ActivityInstance{}, false, fmt.Errorf("fake %T creates no template-backed instance", p.fake)
	}
	row := legacyInstanceOfInput(input)
	inserted, err := fake.CreateTemplateBackedIfAbsent(ctx, row)
	p.created[row.ID] = row
	return timetable.ActivityInstance{ID: row.ID}, inserted, err
}

func (p legacyInstancePlans) UpdateActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	fake, ok := p.fake.(interface {
		Update(context.Context, *scheduleModels.ActivityInstance) error
	})
	if !ok {
		return timetable.ActivityInstance{}, fmt.Errorf("fake %T updates no instance", p.fake)
	}
	row := legacyInstanceOfInput(input)
	row.ID = id
	return timetable.ActivityInstance{ID: id}, fake.Update(ctx, row)
}

func (p legacyInstancePlans) PatchActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput, columns []string) (int64, error) {
	row := legacyInstanceOfInput(input)
	row.ID = id
	return legacyInstanceColumns(ctx, p.fake, row, columns...)
}

func (p legacyInstancePlans) DeleteActivityInstance(ctx context.Context, id int64) error {
	fake, ok := p.fake.(interface {
		Delete(context.Context, int64) error
	})
	if !ok {
		return fmt.Errorf("fake %T deletes no instance", p.fake)
	}
	return fake.Delete(ctx, id)
}

func (p legacyInstancePlans) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (int64, error) {
	fake, ok := p.fake.(interface {
		DeletePlannedNonSpontaneousInWindow(context.Context, calendar.Date, *calendar.Date, *int64, bool) (int64, error)
	})
	if !ok {
		return 0, fmt.Errorf("fake %T deletes no planned instances", p.fake)
	}
	var until *calendar.Date
	if to != nil {
		day := calendar.Date(*to)
		until = &day
	}
	return fake.DeletePlannedNonSpontaneousInWindow(ctx, calendar.Date(from), until, groupID, preserveDeviations)
}

func (p legacyInstancePlans) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	fake, ok := p.fake.(interface {
		DeletePlannedMaterializedWeekendInstances(context.Context, int64, []int) (int64, error)
	})
	if !ok {
		return 0, fmt.Errorf("fake %T deletes no weekend instances", p.fake)
	}
	return fake.DeletePlannedMaterializedWeekendInstances(ctx, groupID, weekdays)
}

func (p legacyInstancePlans) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (int64, error) {
	fake, ok := p.fake.(interface {
		PropagateListKindToFutureInstances(context.Context, int64, *string, *string, calendar.Date) (int64, error)
	})
	if !ok {
		return 0, fmt.Errorf("fake %T propagates no list kind", p.fake)
	}
	return fake.PropagateListKindToFutureInstances(ctx, groupID, previousKind, newKind, calendar.Date(after))
}

func (p legacyInstancePlans) MaxActivityInstanceID(ctx context.Context) (int64, error) {
	fake, ok := p.fake.(interface {
		MaxID(context.Context) (int64, error)
	})
	if !ok {
		return 0, fmt.Errorf("fake %T has no max id", p.fake)
	}
	return fake.MaxID(ctx)
}

type legacyInstanceSessions struct {
	ports.InstanceSessions
	fake any
}

func (s legacyInstanceSessions) DiscardActivitySession(context.Context, int64) error {
	return nil
}

func (s legacyInstanceSessions) RecordActivitySessionCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	fake, ok := s.fake.(interface {
		MarkCompleted(context.Context, int64, time.Time) error
	})
	if !ok {
		return fmt.Errorf("fake %T marks nothing completed", s.fake)
	}
	return fake.MarkCompleted(ctx, instanceID, completedAt)
}

func (s legacyInstanceSessions) CompleteActivitySessionsByGroups(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	fake, ok := s.fake.(interface {
		CompleteActiveByActiveGroupIDs(context.Context, []int64, time.Time) (int64, error)
	})
	if !ok {
		return 0, fmt.Errorf("fake %T completes no sessions", s.fake)
	}
	return fake.CompleteActiveByActiveGroupIDs(ctx, activeGroupIDs, completedAt)
}

func legacyInstanceColumns(ctx context.Context, fake any, row *scheduleModels.ActivityInstance, columns ...string) (int64, error) {
	updater, ok := fake.(interface {
		UpdateColumns(context.Context, *scheduleModels.ActivityInstance, ...string) (int64, error)
	})
	if !ok {
		return 0, fmt.Errorf("fake %T updates no columns", fake)
	}
	return updater.UpdateColumns(ctx, row, columns...)
}

func legacyInstanceOfInput(input timetable.ActivityInstanceInput) *scheduleModels.ActivityInstance {
	start, _ := time.Parse("15:04:05", input.StartTime)
	end, _ := time.Parse("15:04:05", input.EndTime)
	return &scheduleModels.ActivityInstance{
		StartTime: start, EndTime: end,
		Date: calendar.Date(input.Date), ActivityGroupID: input.ActivityGroupID, CalendarPeriodID: input.CalendarPeriodID,
		Title: input.Title, Description: input.Description, RoomID: input.RoomID, RequiredStaff: input.RequiredStaff,
		Status: input.Status, ListKind: input.ListKind, IsSpontaneous: input.IsSpontaneous, UnderstaffedAck: input.UnderstaffedAck,
		UnderstaffedNote: input.UnderstaffedNote, CancelReason: input.CancelReason, Notes: input.Notes,
		IdempotencyKey: input.IdempotencyKey, IdempotencyFingerprint: input.IdempotencyFingerprint, CreatedBy: input.CreatedBy,
	}
}
