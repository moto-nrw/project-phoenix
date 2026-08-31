package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// AutoStartService starts due planned timetable instances when a tenant opts
// into timetable.auto_start_planned.
type AutoStartService interface {
	RunForTenant(ctx context.Context, now time.Time) (*AutoStartResult, error)
}

// AutoStartResult summarizes one scheduler tick for a single tenant.
type AutoStartResult struct {
	Checked             int
	Started             int
	SkippedBeforeWindow int
	SkippedAfterWindow  int
	SkippedNoStaff      int
	SkippedConflict     int
	SkippedMoved        int
	SkippedNonPlanned   int
	Failed              int
	DurationMS          int64
}

// AutoStartConflictDetector is a test seam for the otherwise repo-heavy
// DetectStartConflicts helper. Production wiring leaves it unset.
type AutoStartConflictDetector func(
	ctx context.Context,
	deps ConflictDependencies,
	instance *scheduleModel.ActivityInstance,
	logger *slog.Logger,
) []InstanceConflictWarning

// AutoStartDependencies groups the collaborators required for automatic starts.
type AutoStartDependencies struct {
	InstanceRepo      scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	InstanceStudents  scheduleModel.InstanceStudentRepository
	InstanceService   InstanceService
	RoomRepo          facilitiesModel.RoomRepository
	ActiveGroupRepo   activeModel.GroupRepository
	SupervisorRepo    activeModel.GroupSupervisorRepository
	VisitRepo         activeModel.VisitRepository
	ConflictDetector  AutoStartConflictDetector
	Logger            *slog.Logger
}

type autoStartService struct {
	AutoStartDependencies
	conflictDeps ConflictDependencies
}

// NewAutoStartService creates the tenant-scoped auto-start service. It is
// deliberately conservative: it only starts currently-running planned slots
// with at least one non-absent staff assignment and no preflight conflicts.
func NewAutoStartService(deps AutoStartDependencies) AutoStartService {
	if deps.InstanceRepo == nil {
		panic("schedule auto-start: InstanceRepo is required")
	}
	if deps.InstanceStaffRepo == nil {
		panic("schedule auto-start: InstanceStaffRepo is required")
	}
	if deps.InstanceService == nil {
		panic("schedule auto-start: InstanceService is required")
	}
	if deps.RoomRepo == nil {
		panic("schedule auto-start: RoomRepo is required")
	}
	if deps.ConflictDetector == nil {
		if deps.ActiveGroupRepo == nil {
			panic("schedule auto-start: ActiveGroupRepo is required")
		}
		if deps.SupervisorRepo == nil {
			panic("schedule auto-start: SupervisorRepo is required")
		}
		if deps.VisitRepo == nil {
			panic("schedule auto-start: VisitRepo is required")
		}
		if deps.InstanceStudents == nil {
			panic("schedule auto-start: InstanceStudents is required")
		}
		deps.ConflictDetector = DetectStartConflicts
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &autoStartService{
		AutoStartDependencies: deps,
		conflictDeps: ConflictDependencies{
			GroupRepo:         deps.ActiveGroupRepo,
			SupervisorRepo:    deps.SupervisorRepo,
			VisitRepo:         deps.VisitRepo,
			InstanceRepo:      deps.InstanceRepo,
			InstanceStaffRepo: deps.InstanceStaffRepo,
			InstanceStudents:  deps.InstanceStudents,
		},
	}
}

func (s *autoStartService) RunForTenant(ctx context.Context, now time.Time) (*AutoStartResult, error) {
	startedAt := time.Now()
	result := &AutoStartResult{}
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()

	today := timezone.DateFromTime(now)
	instances, err := s.InstanceRepo.FindByTenantAndDate(ctx, today)
	if err != nil {
		return result, fmt.Errorf("load today's activity instances: %w", err)
	}

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
	rooms, err := s.RoomRepo.FindByIDs(ctx, roomIDs)
	if err != nil {
		return result, fmt.Errorf("load rooms for auto-start: %w", err)
	}
	roomNames := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		if room != nil {
			roomNames[room.ID] = room.Name
		}
	}
	for roomID := range plannedRoomIDs {
		if _, found := roomNames[roomID]; !found {
			return result, fmt.Errorf("load rooms for auto-start: room %d not found", roomID)
		}
	}
	staffCounts, err := s.InstanceStaffRepo.CountNonAbsentByInstanceIDs(ctx, plannedIDs)
	if err != nil {
		return result, fmt.Errorf("count assigned staff for auto-start: %w", err)
	}

	for _, inst := range instances {
		result.Checked++
		if inst.Status != scheduleModel.InstanceStatusPlanned {
			result.SkippedNonPlanned++
			continue
		}

		instanceStart := autoStartCombineDayAndTime(today, inst.StartTime)
		if now.Before(instanceStart) {
			result.SkippedBeforeWindow++
			continue
		}
		instanceEnd := autoStartCombineDayAndTime(today, inst.EndTime)
		if !now.Before(instanceEnd) {
			result.SkippedAfterWindow++
			continue
		}
		if staffCounts[inst.ID] < 1 {
			result.SkippedNoStaff++
			continue
		}

		warnings := s.ConflictDetector(ctx, s.conflictDeps, inst, s.Logger)
		if len(warnings) > 0 {
			result.SkippedConflict++
			s.Logger.Warn("auto-start skipped planned instance with conflicts",
				slog.Int64("instance_id", inst.ID),
				slog.Int("warning_count", len(warnings)),
			)
			continue
		}

		if _, err := s.InstanceService.Start(ctx, inst.ID, 0); err != nil {
			// A concurrent admin PUT moved this block to another day between the
			// batch read and Start's locked reload. That is benign: the move is
			// committed, and the next scheduler tick re-reads and starts the block
			// on its real day. Skip it rather than aborting the whole batch (#1840).
			if errors.Is(err, ErrInstanceMoved) {
				result.SkippedMoved++
				s.Logger.Debug("auto-start skipped concurrently-moved instance",
					slog.Int64("instance_id", inst.ID),
				)
				continue
			}
			result.Failed++
			return result, fmt.Errorf("auto-start instance %d: %w", inst.ID, err)
		}
		result.Started++
	}

	return result, nil
}

func autoStartCombineDayAndTime(day timezone.Date, tod time.Time) time.Time {
	return time.Date(
		day.Year(),
		day.Month(),
		day.Day(),
		tod.Hour(),
		tod.Minute(),
		tod.Second(),
		tod.Nanosecond(),
		time.Local,
	)
}
