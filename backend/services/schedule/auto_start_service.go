package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
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
	ActiveGroupRepo   activeModel.GroupRepository
	SupervisorRepo    activeModel.GroupSupervisorRepository
	VisitRepo         activeModel.VisitRepository
	ConflictDetector  AutoStartConflictDetector
	Logger            *slog.Logger
}

type autoStartService struct {
	instanceRepo      scheduleModel.ActivityInstanceRepository
	instanceStaffRepo scheduleModel.InstanceStaffRepository
	instanceService   InstanceService
	conflictDeps      ConflictDependencies
	detectConflicts   AutoStartConflictDetector
	logger            *slog.Logger
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
	detector := deps.ConflictDetector
	if detector == nil {
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
		detector = DetectStartConflicts
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &autoStartService{
		instanceRepo:      deps.InstanceRepo,
		instanceStaffRepo: deps.InstanceStaffRepo,
		instanceService:   deps.InstanceService,
		conflictDeps: ConflictDependencies{
			GroupRepo:         deps.ActiveGroupRepo,
			SupervisorRepo:    deps.SupervisorRepo,
			VisitRepo:         deps.VisitRepo,
			InstanceStaffRepo: deps.InstanceStaffRepo,
			InstanceStudents:  deps.InstanceStudents,
		},
		detectConflicts: detector,
		logger:          logger,
	}
}

func (s *autoStartService) RunForTenant(ctx context.Context, now time.Time) (*AutoStartResult, error) {
	startedAt := time.Now()
	result := &AutoStartResult{}
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()

	today := timezone.DateFromTime(now)
	instances, err := s.instanceRepo.FindByTenantAndDate(ctx, today)
	if err != nil {
		return result, fmt.Errorf("load today's activity instances: %w", err)
	}

	plannedIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == scheduleModel.InstanceStatusPlanned {
			plannedIDs = append(plannedIDs, inst.ID)
		}
	}
	staffCounts, err := s.instanceStaffRepo.CountNonAbsentByInstanceIDs(ctx, plannedIDs)
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

		warnings := s.detectConflicts(ctx, s.conflictDeps, inst, s.logger)
		if len(warnings) > 0 {
			result.SkippedConflict++
			s.logger.Warn("auto-start skipped planned instance with conflicts",
				slog.Int64("instance_id", inst.ID),
				slog.Int("warning_count", len(warnings)),
			)
			continue
		}

		if _, err := s.instanceService.Start(ctx, inst.ID, 0); err != nil {
			result.Failed++
			return result, fmt.Errorf("auto-start instance %d: %w", inst.ID, err)
		}
		result.Started++
	}

	return result, nil
}

func autoStartCombineDayAndTime(day timezone.Date, tod time.Time) time.Time {
	return time.Date(
		day.Year,
		day.Month,
		day.Day,
		tod.Hour(),
		tod.Minute(),
		tod.Second(),
		tod.Nanosecond(),
		time.Local,
	)
}
