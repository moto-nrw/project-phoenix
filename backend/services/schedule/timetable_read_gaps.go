package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// UnderstaffedInstance is one instance in the requested window that is
// understaffed (see IsUnderstaffed), with its staff-row counts already
// computed. The instance carries its own UnderstaffedAck flag and note, so the
// caller partitions open gaps from acknowledged ones without another lookup.
type UnderstaffedInstance struct {
	Instance           *scheduleModel.ActivityInstance
	AssignedStaffCount int
	AbsentStaffCount   int
	PresentStaffCount  int
	PlannedStaffCount  int
}

// ComputeGaps loads every planned/active instance in the inclusive [from, to]
// window, bulk-loads their staff rows, and returns the subset that is
// understaffed with per-instance counts. completed/cancelled instances are
// history and never surfaced. One instances query plus one bulk staff query —
// no per-instance N+1.
func (s *TimetableDataService) ComputeGaps(ctx context.Context, from, to timezone.Date) ([]UnderstaffedInstance, error) {
	instances, err := s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	candidates := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	candidateIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned &&
			inst.Status != scheduleModel.InstanceStatusActive {
			continue
		}
		candidates = append(candidates, inst)
		candidateIDs = append(candidateIDs, inst.ID)
	}

	allRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}
	rowsByInstance := make(map[int64][]*scheduleModel.InstanceStaff, len(candidateIDs))
	for _, row := range allRows {
		rowsByInstance[row.InstanceID] = append(rowsByInstance[row.InstanceID], row)
	}

	gaps := make([]UnderstaffedInstance, 0)
	for _, inst := range candidates {
		rows := rowsByInstance[inst.ID]
		if !IsUnderstaffed(rows) {
			continue
		}
		gap := UnderstaffedInstance{Instance: inst, AssignedStaffCount: len(rows)}
		for _, row := range rows {
			if row.IsAbsent {
				gap.AbsentStaffCount++
			} else {
				gap.PresentStaffCount++
			}
			if !row.IsSubstitute {
				gap.PlannedStaffCount++
			}
		}
		gaps = append(gaps, gap)
	}
	return gaps, nil
}
