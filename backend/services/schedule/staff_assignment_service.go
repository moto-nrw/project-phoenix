package schedule

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// StaffAssignment is one Betreuungsplan block a staff member is planned into on
// a concrete date: where (room), what (activity title/group), when, and the
// Vertretungsplan facts (substitute / absent / block cancelled / understaffed,
// #1840). It is the "Ort/Aufgabe" the Dienstplan shift alone cannot express —
// a shift says *when* the person works, an assignment says *what* within it
// (#1844). Deliberately carries no student data (GDPR: no child names in
// time-tracking lists).
type StaffAssignment struct {
	InstanceID int64
	Title      string
	// GroupName is the activity/Betreuungsgruppe name, nil for a spontaneous
	// instance with no template.
	GroupName *string
	RoomName  string
	Date      timezone.Date
	// StartTime/EndTime are wall-clock (TIME columns), normalized for the caller.
	StartTime time.Time
	EndTime   time.Time
	Status    string
	// Cancelled is a convenience mirror of Status == cancelled: the block does
	// not take place ("fällt aus").
	Cancelled bool
	IsPrimary bool
	// IsSubstitute marks this staff member as a stand-in for the block, IsAbsent
	// that they were pulled from it; both come from the Vertretungsplan (#1840).
	IsSubstitute  bool
	IsAbsent      bool
	AbsenceReason *string
	// CancelReason / UnderstaffedAck surface the block-level Vertretungsplan
	// state so the staff member sees *why* their planned block changed.
	CancelReason    *string
	UnderstaffedAck bool
}

// StaffAssignmentService resolves the Betreuungsplan blocks a staff member is
// planned into for a date range (self-service "Mein Tag", #1844).
type StaffAssignmentService interface {
	// ListAssignmentsForStaff returns the staff member's own activity-instance
	// assignments in [from, to], enriched with room and activity names and the
	// Vertretungsplan flags, ordered by date then start time.
	ListAssignmentsForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAssignment, error)
}

// StaffAssignmentDependencies wires the read repositories behind the service.
type StaffAssignmentDependencies struct {
	InstanceStaffRepo    scheduleModels.InstanceStaffRepository
	ActivityInstanceRepo scheduleModels.ActivityInstanceRepository
	RoomRepo             facilitiesModels.RoomRepository
	ActivityGroupRepo    activitiesModels.GroupRepository
}

type staffAssignmentService struct {
	deps   StaffAssignmentDependencies
	logger *slog.Logger
}

// NewStaffAssignmentService creates the service.
func NewStaffAssignmentService(deps StaffAssignmentDependencies, logger *slog.Logger) StaffAssignmentService {
	return &staffAssignmentService{deps: deps, logger: logger}
}

func (s *staffAssignmentService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *staffAssignmentService) ListAssignmentsForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAssignment, error) {
	if staffID <= 0 {
		return nil, fmt.Errorf("%w: staff ID is required", ErrShiftInvalid)
	}
	if err := validateShiftRange(from, to); err != nil {
		return nil, err
	}

	// One tenant-scoped query for every instance in the window, mapped by ID.
	// The staff view fetches a single day, so this is a handful of rows; the
	// 62-day cap in validateShiftRange bounds the worst case.
	instances, err := s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return []*StaffAssignment{}, nil
	}
	instanceByID := make(map[int64]*scheduleModels.ActivityInstance, len(instances))
	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		instanceByID[inst.ID] = inst
		instanceIDs = append(instanceIDs, inst.ID)
	}

	// All staff rows for those instances in one IN query, then keep only ours.
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, err
	}
	mine := make([]*scheduleModels.InstanceStaff, 0, len(staffRows))
	for _, row := range staffRows {
		if row.StaffID == staffID {
			mine = append(mine, row)
		}
	}
	if len(mine) == 0 {
		return []*StaffAssignment{}, nil
	}

	roomNames := s.resolveRoomNames(ctx, mine, instanceByID)
	groupNames := s.groupNameResolver(ctx)

	assignments := make([]*StaffAssignment, 0, len(mine))
	for _, row := range mine {
		inst := instanceByID[row.InstanceID]
		if inst == nil {
			continue
		}
		roomID := inst.RoomID
		if row.RoomID != nil {
			roomID = *row.RoomID
		}
		a := &StaffAssignment{
			InstanceID:      inst.ID,
			Title:           inst.Title,
			GroupName:       groupNames(inst.ActivityGroupID),
			RoomName:        roomNames[roomID],
			Date:            inst.Date,
			StartTime:       timezone.WallClock(inst.StartTime),
			EndTime:         timezone.WallClock(inst.EndTime),
			Status:          inst.Status,
			Cancelled:       inst.Status == scheduleModels.InstanceStatusCancelled,
			IsPrimary:       row.IsPrimary,
			IsSubstitute:    row.IsSubstitute,
			IsAbsent:        row.IsAbsent,
			AbsenceReason:   row.AbsenceReason,
			CancelReason:    inst.CancelReason,
			UnderstaffedAck: inst.UnderstaffedAck,
		}
		assignments = append(assignments, a)
	}

	slices.SortFunc(assignments, func(a, b *StaffAssignment) int {
		if c := a.Date.Compare(b.Date); c != 0 {
			return c
		}
		return a.StartTime.Compare(b.StartTime)
	})
	return assignments, nil
}

// resolveRoomNames batch-loads the rooms referenced by the assignments (either
// the instance's primary room or a per-row override) into an id→name map. A
// lookup failure logs and yields an empty name rather than failing the list.
func (s *staffAssignmentService) resolveRoomNames(ctx context.Context, rows []*scheduleModels.InstanceStaff, instanceByID map[int64]*scheduleModels.ActivityInstance) map[int64]string {
	idSet := make(map[int64]struct{})
	for _, row := range rows {
		if inst := instanceByID[row.InstanceID]; inst != nil {
			idSet[inst.RoomID] = struct{}{}
		}
		if row.RoomID != nil {
			idSet[*row.RoomID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	names := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return names
	}
	rooms, err := s.deps.RoomRepo.FindByIDs(ctx, ids)
	if err != nil {
		s.getLogger().WarnContext(ctx, "could not resolve room names for staff assignments, names omitted",
			slog.String("error", err.Error()),
		)
		return names
	}
	for _, room := range rooms {
		names[room.ID] = room.Name
	}
	return names
}

// groupNameResolver returns a memoized lookup from an optional activity-group id
// to its name. Spontaneous instances (nil group) return nil. A lookup error is
// logged once per id and treated as "no name".
func (s *staffAssignmentService) groupNameResolver(ctx context.Context) func(*int64) *string {
	cache := make(map[int64]*string)
	return func(groupID *int64) *string {
		if groupID == nil {
			return nil
		}
		if name, ok := cache[*groupID]; ok {
			return name
		}
		group, err := s.deps.ActivityGroupRepo.FindByID(ctx, *groupID)
		if err != nil {
			s.getLogger().WarnContext(ctx, "could not resolve activity group name for staff assignment",
				slog.Int64("activity_group_id", *groupID),
				slog.String("error", err.Error()),
			)
			cache[*groupID] = nil
			return nil
		}
		name := group.Name
		cache[*groupID] = &name
		return &name
	}
}
