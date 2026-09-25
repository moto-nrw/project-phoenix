package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// InstanceAutoStartDependencies are the collaborators of the automatic
// start. Conflicts is the owner's start check (#3550), the one the manual
// start runs too.
type InstanceAutoStartDependencies struct {
	InstanceRepo      scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	Lifecycle         timetable.InstanceLifecycle
	Rooms             LifecycleRooms
	Conflicts         timetable.StartConflictQuery
	Logger            *slog.Logger
}

type instanceAutoStart struct {
	InstanceAutoStartDependencies
}

// NewInstanceAutoStart composes the tenant-scoped auto-start. It is
// deliberately conservative: it only starts running planned slots with at
// least one present staff assignment and no start conflicts.
func NewInstanceAutoStart(deps InstanceAutoStartDependencies) (timetable.InstanceAutoStart, error) {
	if deps.InstanceRepo == nil || deps.InstanceStaffRepo == nil || deps.Lifecycle == nil || deps.Rooms == nil || deps.Conflicts == nil {
		return nil, errors.New("timetable auto-start: required dependency is nil")
	}
	deps.Logger = orDefaultLogger(deps.Logger)
	return &instanceAutoStart{InstanceAutoStartDependencies: deps}, nil
}

// RunForTenant starts every due planned block of today for the tenant in
// the context.
func (s *instanceAutoStart) RunForTenant(ctx context.Context, now time.Time) (*timetable.AutoStartResult, error) {
	startedAt := time.Now()
	result := &timetable.AutoStartResult{}
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()
	today := timezone.DateFromTime(now)
	instances, err := s.InstanceRepo.FindByTenantAndDate(ctx, scheduleModel.Date(today))
	if err != nil {
		return result, fmt.Errorf("load today's activity instances: %w", err)
	}
	staffCounts, err := s.plannedStaffCounts(ctx, instances)
	if err != nil {
		return result, err
	}
	for _, inst := range instances {
		result.Checked++
		if err := s.startIfDue(ctx, inst, today, now, staffCounts, result); err != nil {
			return result, err
		}
	}
	return result, nil
}

// plannedStaffCounts verifies the planned blocks' rooms and counts their
// present staff. A planned block whose room is gone fails the tick.
func (s *instanceAutoStart) plannedStaffCounts(ctx context.Context, instances []*scheduleModel.ActivityInstance) (map[int64]int, error) {
	plannedIDs := make([]int64, 0, len(instances))
	plannedRoomIDs := make(map[int64]struct{})
	for _, inst := range instances {
		if inst.Status == scheduleModel.InstanceStatusPlanned {
			plannedIDs = append(plannedIDs, inst.ID)
			plannedRoomIDs[inst.RoomID] = struct{}{}
		}
	}
	roomIDs := make([]int64, 0, len(plannedRoomIDs))
	for roomID := range plannedRoomIDs {
		roomIDs = append(roomIDs, roomID)
	}
	roomNames, err := s.Rooms.RoomNamesByID(ctx, roomIDs)
	if err != nil {
		return nil, fmt.Errorf("load rooms for auto-start: %w", err)
	}
	for roomID := range plannedRoomIDs {
		if _, found := roomNames[roomID]; !found {
			return nil, fmt.Errorf("load rooms for auto-start: room %d not found", roomID)
		}
	}
	staffCounts, err := s.InstanceStaffRepo.CountNonAbsentByInstanceIDs(ctx, plannedIDs)
	if err != nil {
		return nil, fmt.Errorf("count assigned staff for auto-start: %w", err)
	}
	return staffCounts, nil
}

// startIfDue starts one block when it is planned, inside its window,
// staffed and free of start conflicts. Only a failure the next tick cannot
// resolve aborts the batch.
func (s *instanceAutoStart) startIfDue(
	ctx context.Context, inst *scheduleModel.ActivityInstance, today timezone.Date, now time.Time, staffCounts map[int64]int, result *timetable.AutoStartResult,
) error {
	if !autoStartEligible(inst, today, now, staffCounts, result) {
		return nil
	}
	warnings, err := detectStartConflicts(ctx, s.Conflicts, inst)
	if err != nil {
		result.Failed++
		return fmt.Errorf("auto-start instance %d: detect conflicts: %w", inst.ID, err)
	}
	if len(warnings) > 0 {
		result.SkippedConflict++
		s.Logger.Warn("auto-start skipped planned instance with conflicts",
			slog.Int64("instance_id", inst.ID),
			slog.Int("warning_count", len(warnings)),
		)
		return nil
	}
	if _, err := s.Lifecycle.Start(ctx, inst.ID, 0); err != nil {
		// A concurrent edit moved the block between the batch read and the
		// locked reload. The move is committed and the next tick starts the
		// block on its real day, so skip it rather than abort (#1840).
		if errors.Is(err, timetable.ErrInstanceMoved) {
			result.SkippedMoved++
			s.Logger.Debug("auto-start skipped concurrently-moved instance",
				slog.Int64("instance_id", inst.ID),
			)
			return nil
		}
		result.Failed++
		return fmt.Errorf("auto-start instance %d: %w", inst.ID, err)
	}
	result.Started++
	return nil
}

func autoStartEligible(inst *scheduleModel.ActivityInstance, today timezone.Date, now time.Time, staffCounts map[int64]int, result *timetable.AutoStartResult) bool {
	switch {
	case inst.Status != scheduleModel.InstanceStatusPlanned:
		result.SkippedNonPlanned++
	case now.Before(autoStartCombineDayAndTime(today, inst.StartTime)):
		result.SkippedBeforeWindow++
	case !now.Before(autoStartCombineDayAndTime(today, inst.EndTime)):
		result.SkippedAfterWindow++
	case staffCounts[inst.ID] < 1:
		result.SkippedNoStaff++
	default:
		return true
	}
	return false
}

func autoStartCombineDayAndTime(day timezone.Date, tod time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), time.Local)
}
