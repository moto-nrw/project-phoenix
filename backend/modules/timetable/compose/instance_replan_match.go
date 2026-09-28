package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// matchRegeneratedInstances finds the freshly materialized planned
// occurrences the snapshots reapply to. When the group had exactly ONE
// planned occurrence that date before the re-plan, a lone survivor is
// matched even with a changed start time, so the deviation follows the
// moved block. On a multi-slot day only the original start time matches,
// so overrides of a deleted slot are never merged onto a survivor (#1840).
func (s *InstanceLifecycleService) matchRegeneratedInstances(
	ctx context.Context, snapshots []deviationSnapshot, occurrences map[groupDay]int, targetActivityGroupID *int64,
) ([]*scheduleModel.ActivityInstance, error) {
	matches := make([]*scheduleModel.ActivityInstance, len(snapshots))
	if len(snapshots) == 0 {
		return matches, nil
	}
	groupIDs, dates := regeneratedMatchKeys(snapshots, targetActivityGroupID)
	options := modelBase.NewQueryOptions()
	options.Filter = modelBase.NewFilter().
		In("activity_group_id", anyArgs(groupIDs)...).
		In("date", anyArgs(dates)...).
		Equal("status", scheduleModel.InstanceStatusPlanned).
		Equal("is_spontaneous", false)
	candidates, err := legacyList[*scheduleModel.ActivityInstance](ctx, s.deps.InstanceRepo, options)
	if err != nil {
		return nil, err
	}
	byGroupDay := indexRegeneratedCandidates(candidates)
	for i, snap := range snapshots {
		groupID := snap.activityGroupID
		if targetActivityGroupID != nil {
			groupID = *targetActivityGroupID
		}
		sole := occurrences[groupDay{snap.activityGroupID, snap.date}] == 1
		matches[i] = matchRegeneratedCandidate(byGroupDay[groupDay{groupID, snap.date}], snap.startTime, sole)
	}
	return matches, nil
}

func regeneratedMatchKeys(snapshots []deviationSnapshot, targetActivityGroupID *int64) ([]int64, []timezone.Date) {
	groupIDs := make([]int64, 0, len(snapshots))
	dates := make([]timezone.Date, 0, len(snapshots))
	seenGroups := make(map[int64]bool)
	seenDates := make(map[timezone.Date]bool)
	for _, snap := range snapshots {
		groupID := snap.activityGroupID
		if targetActivityGroupID != nil {
			groupID = *targetActivityGroupID
		}
		if !seenGroups[groupID] {
			seenGroups[groupID] = true
			groupIDs = append(groupIDs, groupID)
		}
		if !seenDates[snap.date] {
			seenDates[snap.date] = true
			dates = append(dates, snap.date)
		}
	}
	return groupIDs, dates
}

func indexRegeneratedCandidates(candidates []*scheduleModel.ActivityInstance) map[groupDay][]*scheduleModel.ActivityInstance {
	byGroupDay := make(map[groupDay][]*scheduleModel.ActivityInstance)
	for _, candidate := range candidates {
		if candidate.ActivityGroupID != nil {
			key := groupDay{*candidate.ActivityGroupID, timezone.Date(candidate.Date)}
			byGroupDay[key] = append(byGroupDay[key], candidate)
		}
	}
	return byGroupDay
}

func matchRegeneratedCandidate(candidates []*scheduleModel.ActivityInstance, startTime string, sole bool) *scheduleModel.ActivityInstance {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 && sole {
		return candidates[0]
	}
	for _, candidate := range candidates {
		if formatTimeOfDay(candidate.StartTime) == startTime {
			return candidate
		}
	}
	return nil
}

// seriesDeviationPreserver serves the template split's deviation port from
// the lifecycle's snapshot and reapply machinery.
type seriesDeviationPreserver struct {
	lifecycle *InstanceLifecycleService
}

func (p seriesDeviationPreserver) LockDeviationDays(ctx context.Context, tenantID int64, from, to timezone.Date) error {
	return p.lifecycle.acquireSubstituteDayLocks(ctx, tenantID, from, to)
}

func (p seriesDeviationPreserver) SnapshotDeviations(ctx context.Context, from, to timezone.Date, templateID int64) (PreservedDeviations, error) {
	id := templateID
	snapshots, occurrences, err := p.lifecycle.snapshotDeviations(ctx, from, to, &id)
	if err != nil {
		return nil, err
	}
	return seriesDeviationSnapshot{lifecycle: p.lifecycle, snapshots: snapshots, occurrences: occurrences}, nil
}

// seriesDeviationSnapshot is one snapshot of SnapshotDeviations.
type seriesDeviationSnapshot struct {
	lifecycle   *InstanceLifecycleService
	snapshots   []deviationSnapshot
	occurrences map[groupDay]int
}

func (s seriesDeviationSnapshot) Count() int { return len(s.snapshots) }

func (s seriesDeviationSnapshot) Reapply(ctx context.Context, templateID int64, actorAccountID *int64) (int, error) {
	id := templateID
	return s.lifecycle.reapplyDeviations(ctx, s.snapshots, s.occurrences, &id, actorAccountID)
}
