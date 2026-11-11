package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
// Everything runs in one tenant transaction behind the recurrence gate and
// the ascending day locks ReplanWeek takes, so a concurrent move into the
// range or a materialization run waits.
func (s *InstanceLifecycleService) BulkCancelPlanned(ctx context.Context, from, to calendar.Date, opts timetable.BulkCancelOptions, actorAccountID *int64) (*timetable.BulkCancelResult, error) {
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
		// The recurrence gate before the days: the deletes write slot
		// exceptions a running materialization must see (#6499).
		if err := s.lockRecurrence(ctx, "bulk cancel"); err != nil {
			return nil, err
		}
		if err := s.LockDeviationDays(ctx, tenant.FromContext(ctx), from, to); err != nil {
			return nil, &ports.ScheduleError{Op: "bulk cancel: lock days", Err: err}
		}
	}
	candidates, err := s.bulkCancelCandidates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	today := calendar.DateFromTime(s.now()).String()
	ids, result := timetable.SelectBulkCancel(candidates, from.String(), to.String(), today, opts)
	if opts.DryRun {
		return &result, nil
	}
	if err := s.cancelAndDeleteSelected(ctx, bulkCancelledBlocks(candidates, ids), actorAccountID); err != nil {
		return nil, err
	}
	return &result, nil
}

// cancelAndDeleteSelected locks the people of every selected future block in
// one ascending batch (#6702) and then cancels and deletes the blocks.
func (s *InstanceLifecycleService) cancelAndDeleteSelected(ctx context.Context, blocks []cancelledBlock, actorAccountID *int64) error {
	if err := s.lockCancelledBlockStaff(ctx, blocks, "bulk cancel"); err != nil {
		return err
	}
	reason := bulkCancelReason
	for _, block := range blocks {
		if _, err := s.cancel(ctx, block.id, &reason, actorAccountID, false); err != nil {
			return err
		}
		if err := s.deleteInstance(ctx, block.id, bulkCancelReason, false); err != nil {
			return err
		}
	}
	return nil
}

// bulkCancelledBlocks names the selected candidates with their dates.
func bulkCancelledBlocks(candidates []timetable.BulkCancelCandidate, ids []int64) []cancelledBlock {
	selected := make(map[int64]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	blocks := make([]cancelledBlock, 0, len(ids))
	for _, candidate := range candidates {
		if selected[candidate.InstanceID] {
			blocks = append(blocks, cancelledBlock{id: candidate.InstanceID, date: calendar.Date(candidate.Date)})
		}
	}
	return blocks
}

// bulkCancelCandidates loads the occurrences of [from, to] with the closing
// day flag and name of their series: one instance and one group query.
func (s *InstanceLifecycleService) bulkCancelCandidates(ctx context.Context, from, to calendar.Date) ([]timetable.BulkCancelCandidate, error) {
	rows, err := instancesBetween(ctx, s.deps.InstanceRepo.Reads, calendar.Date(from), calendar.Date(to))
	if err != nil {
		return nil, &ports.ScheduleError{Op: "bulk cancel: load instances", Err: err}
	}
	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ActivityGroupID != nil {
			groupIDs = append(groupIDs, *row.ActivityGroupID)
		}
	}
	series := make(map[int64]timetable.BulkCancelCandidate, len(groupIDs))
	if len(groupIDs) > 0 {
		groups, err := legacyGroupsByID(ctx, s.deps.ActivityGroupRepo, groupIDs)
		if err != nil {
			return nil, &ports.ScheduleError{Op: "bulk cancel: load series", Err: err}
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
