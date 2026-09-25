package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// bulkCancelReason is stored on the cancellations and slot exceptions a bulk
// cancellation (#3594) writes, so the change log names where they came from.
const bulkCancelReason = "Termine im Zeitraum abgesagt"

// BulkCancelPlanned cancels and removes the planned occurrences in [from, to]
// from today on (#3594); series that include closing days only on request.
// Each occurrence runs through Cancel (change log entry, SSE) and then the
// DeleteCancelled path (slot exception, so no later materialization or
// re-plan recreates it). Guardians are not notified. opts.DryRun only counts.
// Everything runs in one tenant transaction behind the ascending day locks
// ReplanWeek takes, so a concurrent move into the range waits.
func (s *InstanceLifecycleService) BulkCancelPlanned(ctx context.Context, from, to timezone.Date, opts timetable.BulkCancelOptions, actorAccountID *int64) (*timetable.BulkCancelResult, error) {
	if err := timetable.ValidateBulkCancelRange(from.String(), to.String()); err != nil {
		return nil, err
	}
	if !s.hasTx(ctx) {
		var result *timetable.BulkCancelResult
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var bulkErr error
			result, bulkErr = s.BulkCancelPlanned(txCtx, from, to, opts, actorAccountID)
			return bulkErr
		})
		return result, err
	}
	if !opts.DryRun {
		if err := s.acquireSubstituteDayLocks(ctx, tenant.FromContext(ctx), from, to); err != nil {
			return nil, &ScheduleError{Op: "bulk cancel: lock days", Err: err}
		}
	}
	candidates, err := s.bulkCancelCandidates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	today := timezone.DateFromTime(s.now()).String()
	ids, result := timetable.SelectBulkCancel(candidates, from.String(), to.String(), today, opts)
	if opts.DryRun {
		return &result, nil
	}
	reason := bulkCancelReason
	for _, id := range ids {
		if _, err := s.cancel(ctx, id, &reason, actorAccountID); err != nil {
			return nil, err
		}
		if err := s.deleteInstance(ctx, id, bulkCancelReason, false); err != nil {
			return nil, err
		}
	}
	return &result, nil
}

// bulkCancelCandidates loads the occurrences of [from, to] with the closing
// day flag and name of their series: one instance and one group query.
func (s *InstanceLifecycleService) bulkCancelCandidates(ctx context.Context, from, to timezone.Date) ([]timetable.BulkCancelCandidate, error) {
	rows, err := s.deps.InstanceRepo.FindByTenantAndDateRange(ctx, scheduleModel.Date(from), scheduleModel.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "bulk cancel: load instances", Err: err}
	}
	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ActivityGroupID != nil {
			groupIDs = append(groupIDs, *row.ActivityGroupID)
		}
	}
	series := make(map[int64]timetable.BulkCancelCandidate, len(groupIDs))
	if len(groupIDs) > 0 {
		groups, err := s.deps.ActivityGroupRepo.FindByIDs(ctx, groupIDs)
		if err != nil {
			return nil, &ScheduleError{Op: "bulk cancel: load series", Err: err}
		}
		for _, group := range groups {
			series[group.ID] = timetable.BulkCancelCandidate{SeriesIncludesClosingDays: group.IncludeClosingDays, SeriesName: group.Name}
		}
	}
	candidates := make([]timetable.BulkCancelCandidate, 0, len(rows))
	for _, row := range rows {
		var candidate timetable.BulkCancelCandidate
		if row.ActivityGroupID != nil {
			candidate = series[*row.ActivityGroupID]
		}
		candidate.InstanceID, candidate.Date, candidate.Status = row.ID, row.Date.String(), row.Status
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}
