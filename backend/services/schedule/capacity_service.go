// Package schedule — Betreuungsplan capacity computation (issue #1838).
//
// Computes the Betreuungsschlüssel-derived staffing requirement for a
// materialized Betreuungsplan block (activity instance) and compares it
// against the currently assigned (non-absent) staff. Pure read/compute: this
// service never writes, and it deliberately does not cross-check assigned
// staff against schedule.staff_shifts (Dienstplan) — "assigned" already has
// an established meaning throughout this codebase (instance_staff rows with
// is_absent=false), and Dienstplan-vs-roster consistency is a different,
// independently useful signal (issue #1812), not a staffing-ratio question.
//
// The list endpoints (instances_list.go, templates_list.go) do NOT call this
// service on their hot path — they already have instance_staff/
// instance_students rows loaded per row and apply RequiredStaffForChildren
// directly to the counts they already hold, avoiding an extra DB round-trip
// per list request. This service exists for standalone callers that only
// have an instance ID (e.g. a future detail endpoint, or issue #1839's
// confirm-understaffing workflow).
package schedule

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// defaultChildrenPerStaffRatio mirrors the registry default for
// timetable.children_per_staff_ratio (services/config/defaults/timetable.go)
// and is the fallback used when the settings service is unwired (tests) or a
// tenant override lookup fails.
const defaultChildrenPerStaffRatio = 12

// RequiredStaffForChildren returns the number of staff required to supervise
// childrenCount children at the given ratio (max children per staff member),
// rounded up. ratio < 1 is defensively clamped to 1 so a corrupted override
// can never divide by zero or invert the relationship; the registry
// Validation (1-30) already prevents this in practice.
func RequiredStaffForChildren(childrenCount, ratio int) int {
	if childrenCount <= 0 {
		return 0
	}
	if ratio < 1 {
		ratio = 1
	}
	return (childrenCount + ratio - 1) / ratio
}

// CapacityResult is the required-vs-assigned staff snapshot for one
// Betreuungsplan block (materialized instance).
type CapacityResult struct {
	RequiredStaff int
	AssignedStaff int
	ChildrenCount int
	Ratio         int
}

// Understaffed reports whether fewer staff are assigned than the computed
// requirement.
func (c CapacityResult) Understaffed() bool {
	return c.AssignedStaff < c.RequiredStaff
}

// CapacityDependencies bundles the repositories and settings service
// CapacityService needs. Logger may be nil; getLogger() falls back to
// slog.Default().
type CapacityDependencies struct {
	InstanceStudentRepo scheduleModel.InstanceStudentRepository
	InstanceStaffRepo   scheduleModel.InstanceStaffRepository
	Settings            configSvc.SettingsService
	Logger              *slog.Logger
}

// CapacityService computes required-vs-assigned staffing for Betreuungsplan
// blocks.
type CapacityService interface {
	// ComputeBlockCapacity returns the capacity snapshot for a single
	// instance.
	ComputeBlockCapacity(ctx context.Context, instanceID int64) (CapacityResult, error)

	// ComputeBlockCapacityBulk returns a capacity snapshot per instance ID.
	// Every requested ID is present in the result (zero children/staff for
	// instances with no rows) so callers never need a presence check.
	ComputeBlockCapacityBulk(ctx context.Context, instanceIDs []int64) (map[int64]CapacityResult, error)
}

type capacityService struct {
	deps CapacityDependencies
}

// NewCapacityService creates a new CapacityService.
func NewCapacityService(deps CapacityDependencies) CapacityService {
	return &capacityService{deps: deps}
}

func (s *capacityService) getLogger() *slog.Logger {
	return cmp.Or(s.deps.Logger, slog.Default())
}

func (s *capacityService) ratio(ctx context.Context) int {
	return configSvc.ResolveIntOrDefault(
		ctx,
		s.deps.Settings,
		configModel.KeyTimetableChildrenPerStaffRatio,
		defaultChildrenPerStaffRatio,
		s.getLogger(),
	)
}

func (s *capacityService) ComputeBlockCapacity(ctx context.Context, instanceID int64) (CapacityResult, error) {
	results, err := s.ComputeBlockCapacityBulk(ctx, []int64{instanceID})
	if err != nil {
		return CapacityResult{}, err
	}
	return results[instanceID], nil
}

func (s *capacityService) ComputeBlockCapacityBulk(ctx context.Context, instanceIDs []int64) (map[int64]CapacityResult, error) {
	if len(instanceIDs) == 0 {
		return map[int64]CapacityResult{}, nil
	}

	ratio := s.ratio(ctx)

	childrenCounts, err := s.deps.InstanceStudentRepo.CountNonAbsentByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("count children per instance: %w", err)
	}
	staffCounts, err := s.deps.InstanceStaffRepo.CountNonAbsentByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("count staff per instance: %w", err)
	}

	out := make(map[int64]CapacityResult, len(instanceIDs))
	for _, id := range instanceIDs {
		children := childrenCounts[id]
		out[id] = CapacityResult{
			RequiredStaff: RequiredStaffForChildren(children, ratio),
			AssignedStaff: staffCounts[id],
			ChildrenCount: children,
			Ratio:         ratio,
		}
	}
	return out, nil
}
