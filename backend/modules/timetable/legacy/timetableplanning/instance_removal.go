package timetableplanning

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// bulkCancelReason is stored on the cancellations and slot exceptions a bulk
// cancellation (#3594) writes, so the change log names where they came from.
const bulkCancelReason = "Termine im Zeitraum abgesagt"

// DeleteCancelled permanently removes a planned or cancelled instance.
// Historical name retained for compatibility with existing handlers. Active
// and completed instances stay protected: deleting those would hide live
// sessions or attendance history. For materialized template occurrences,
// write a cancelled activity_exception first so materialization cannot
// resurrect the deleted single occurrence.
func (s *instanceService) DeleteCancelled(ctx context.Context, instanceID int64) error {
	return s.deleteInstance(ctx, instanceID, deletedSlotReason, true)
}

// deleteInstance implements DeleteCancelled. The slot exception covers the
// whole day of the series, so a single delete rejects a series with several
// same-day slots; a bulk cancellation removes all of them and skips that
// check.
func (s *instanceService) deleteInstance(ctx context.Context, instanceID int64, slotReason string, rejectAmbiguous bool) error {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return err
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusCancelled:
		// allowed
	default:
		return fmt.Errorf("%w: cannot delete instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}

	if instance.ActivityGroupID != nil && !instance.IsSpontaneous {
		if rejectAmbiguous {
			if err := s.rejectAmbiguousTemplateDelete(ctx, *instance.ActivityGroupID, timezone.Date(instance.Date)); err != nil {
				return err
			}
		}
		if err := s.ensureCancelledSlotException(ctx, *instance.ActivityGroupID, timezone.Date(instance.Date), slotReason); err != nil {
			return err
		}
	}

	if err := s.deps.InstanceRepo.Delete(ctx, instance.ID); err != nil {
		return &ScheduleError{Op: "delete instance", Err: err}
	}
	s.getLogger().Info("instance deleted",
		slog.Int64("tenant_id", tenant.FromContext(ctx)),
		slog.Int64("instance_id", instance.ID),
		slog.String("date", instance.Date.String()),
		slog.String("status", instance.Status),
	)
	s.broadcastPlannedInstanceChanged(ctx, "instance_delete")
	return nil
}

func (s *instanceService) rejectAmbiguousTemplateDelete(ctx context.Context, activityGroupID int64, date timezone.Date) error {
	rows, err := s.deps.InstanceRepo.FindByActivityGroupAndDate(ctx, activityGroupID, scheduleModel.Date(date))
	if err != nil {
		return &ScheduleError{Op: "delete instance: check same-day template slots", Err: err}
	}
	templateBacked := 0
	for _, row := range rows {
		if row != nil && !row.IsSpontaneous {
			templateBacked++
		}
	}
	if templateBacked > 1 {
		return fmt.Errorf("%w: template has %d same-day slots", ErrAmbiguousTemplateInstanceDelete, templateBacked)
	}
	return nil
}

// BulkCancelPlanned cancels and removes the planned occurrences in [from, to]
// from today on (#3594); series that include closing days only on request.
// Each occurrence runs through Cancel (change log entry, SSE) and then the
// DeleteCancelled path (slot exception, so no later materialization or
// re-plan recreates it). Guardians are not notified. opts.DryRun only counts.
// Everything runs in one tenant transaction behind the ascending day locks
// ReplanWeek takes, so a concurrent move into the range waits.
func (s *instanceService) BulkCancelPlanned(ctx context.Context, from, to timezone.Date, opts timetable.BulkCancelOptions, actorAccountID *int64) (*timetable.BulkCancelResult, error) {
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
		if _, err := s.Cancel(ctx, id, &reason, actorAccountID); err != nil {
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
func (s *instanceService) bulkCancelCandidates(ctx context.Context, from, to timezone.Date) ([]timetable.BulkCancelCandidate, error) {
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
