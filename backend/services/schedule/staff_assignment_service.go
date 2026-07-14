package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
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

	roomNames, err := s.resolveRoomNames(ctx, mine, instanceByID)
	if err != nil {
		return nil, err
	}
	groupNames, err := s.resolveGroupNames(ctx, mine, instanceByID)
	if err != nil {
		return nil, err
	}

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
			GroupName:       groupNameFor(inst.ActivityGroupID, groupNames),
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
// lookup failure propagates: the room is the "Ort" this feature exists to show,
// and these reads run inside TenantTxMiddleware, where a failed statement can
// abort the transaction and make the commit fail after a 200 was written.
func (s *staffAssignmentService) resolveRoomNames(ctx context.Context, rows []*scheduleModels.InstanceStaff, instanceByID map[int64]*scheduleModels.ActivityInstance) (map[int64]string, error) {
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
		return names, nil
	}
	rooms, err := s.deps.RoomRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve room names for staff assignments: %w", err)
	}
	for _, room := range rooms {
		names[room.ID] = room.Name
	}
	return names, nil
}

// resolveGroupNames loads the activity-group name for every group referenced by
// the assignments into an id→name map. A row that is genuinely gone (archived/
// deleted group) is tolerated as "no name"; any other lookup failure propagates
// for the same transaction-abort reason as resolveRoomNames.
func (s *staffAssignmentService) resolveGroupNames(ctx context.Context, rows []*scheduleModels.InstanceStaff, instanceByID map[int64]*scheduleModels.ActivityInstance) (map[int64]string, error) {
	names := make(map[int64]string)
	for _, row := range rows {
		inst := instanceByID[row.InstanceID]
		if inst == nil || inst.ActivityGroupID == nil {
			continue
		}
		groupID := *inst.ActivityGroupID
		if _, ok := names[groupID]; ok {
			continue
		}
		group, err := s.deps.ActivityGroupRepo.FindByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.getLogger().WarnContext(ctx, "activity group referenced by staff assignment no longer exists, name omitted",
					slog.Int64("activity_group_id", groupID),
				)
				continue
			}
			return nil, fmt.Errorf("failed to resolve activity group name for staff assignment: %w", err)
		}
		names[groupID] = group.Name
	}
	return names, nil
}

// groupNameFor maps an optional activity-group id through the resolved name
// map; spontaneous instances (nil group) and vanished groups yield nil.
func groupNameFor(groupID *int64, names map[int64]string) *string {
	if groupID == nil {
		return nil
	}
	name, ok := names[*groupID]
	if !ok {
		return nil
	}
	return &name
}
