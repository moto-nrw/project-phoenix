package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// EndFromDate caps a template from the effective date onward without creating
// a successor ("Dieser und alle folgenden löschen") — intentionally the
// destructive half of Split. Planned non-spontaneous future instances are
// deleted; active/completed/cancelled/spontaneous rows remain as history.
func (s *TemplateSplitService) EndFromDate(ctx context.Context, in timetable.EndTemplateCommand) (*timetable.EndTemplateResult, error) {
	if err := validateTemplateEndInput(in, s.deps.Today()); err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "end template", Err: errors.New("no tenant in context")}
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok && s.runInTx != nil {
		return s.endFromDateWithTransaction(ctx, in, tenantID)
	}
	return s.endFromDateInTransaction(ctx, in, tenantID)
}

func (s *TemplateSplitService) endFromDateWithTransaction(
	ctx context.Context,
	in timetable.EndTemplateCommand,
	tenantID int64,
) (*timetable.EndTemplateResult, error) {
	var result *timetable.EndTemplateResult
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		var innerErr error
		result, innerErr = s.endFromDateInTransaction(txCtx, in, tenantID)
		return innerErr
	})
	return result, err
}

func (s *TemplateSplitService) endFromDateInTransaction(
	ctx context.Context,
	in timetable.EndTemplateCommand,
	tenantID int64,
) (*timetable.EndTemplateResult, error) {
	if err := s.lockSplitRecurrence(ctx, "end template"); err != nil {
		return nil, err
	}
	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return nil, err
	}
	// The raw requested date drives the predecessor cascade below: normalize
	// clamps to THIS segment's valid_from, but "diesen und alle folgenden
	// löschen" from a date inside an earlier bounded segment must also cap that
	// segment (#2187).
	requestedFrom := in.EffectiveDate
	effectiveDate, sourceValidUntil, err := s.normalizeEffectiveDateInSegment(ctx, old.ID, in.EffectiveDate, "end template", true)
	if err != nil {
		return nil, err
	}
	result, err := s.capEndedSegment(ctx, old.ID, effectiveDate, sourceValidUntil)
	if err != nil {
		return nil, err
	}
	if err := s.validateCareOfferingSeries(ctx, old.ID); err != nil {
		return nil, s.deps.CareOfferings.validationError(
			ctx,
			"end template: validate linked care offerings",
			"ending the recurrence is incompatible with an existing care offering",
			err,
		)
	}

	deleted, err := s.deleteEndedInstances(ctx, old.ID, effectiveDate, requestedFrom)
	if err != nil {
		return nil, err
	}
	result.DeletedInstances = int(deleted)

	// Ending a series deletes planned instances outside the CRUD broadcast
	// paths — invalidate the staffing caches (#1844).
	if deleted > 0 {
		announceStaffingChanged(ctx, s.deps.Staffing, s.getLogger(), "template_end")
	}

	s.getLogger().Info("template ended from date",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("template_id", old.ID),
		slog.String("effective_date", effectiveDate.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int64("capped_schedules", result.CappedSchedules),
		slog.Int64("capped_enrollments", result.CappedEnrollments),
		slog.Int64("capped_supervisors", result.CappedSupervisors),
	)
	return result, nil
}

// deleteEndedInstances removes the planned occurrences of the ended segment
// and of every later segment the cascade ends. preserveDeviations=false:
// "end this and all following" must actually remove the whole planned series
// — keeping a deviated row would leave the ended series partly alive (#1840).
func (s *TemplateSplitService) deleteEndedInstances(
	ctx context.Context,
	templateID int64,
	effectiveDate, requestedFrom timezone.Date,
) (int64, error) {
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, scheduleModel.Date(effectiveDate), nil, &templateID, false)
	if err != nil {
		return 0, &ScheduleError{Op: "end template: delete planned instances", Err: err}
	}
	cascadeDeleted, err := s.cascadeEndToSeriesSegments(ctx, templateID, requestedFrom)
	if err != nil {
		return 0, err
	}
	return deleted + cascadeDeleted, nil
}

// capEndedSegment caps the ended segment's schedules, open roster rows and
// the bounded rows that share its end at the effective date.
func (s *TemplateSplitService) capEndedSegment(
	ctx context.Context,
	templateID int64,
	effectiveDate timezone.Date,
	sourceValidUntil *timezone.Date,
) (*timetable.EndTemplateResult, error) {
	segmentEnrollments, segmentSupervisors, err := s.loadSegmentRosterCandidates(ctx, templateID, effectiveDate, sourceValidUntil)
	if err != nil {
		return nil, err
	}
	cappedSchedules, err := s.deps.ScheduleRepo.CapValidUntil(ctx, templateID, effectiveDate.String())
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap schedules", Err: err}
	}
	cappedEnrollments, err := s.deps.EnrollmentRepo.CapActiveByGroup(ctx, templateID, activitiesModel.Date(effectiveDate))
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap enrollments", Err: err}
	}
	cappedSupervisors, err := s.deps.SupervisorRepo.CapActiveByGroup(ctx, templateID, activitiesModel.Date(effectiveDate))
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap supervisors", Err: err}
	}
	cappedBoundedEnrollments, err := s.capBoundedSegmentEnrollments(ctx, segmentEnrollments, sourceValidUntil, effectiveDate)
	if err != nil {
		return nil, err
	}
	cappedBoundedSupervisors, err := s.capBoundedSegmentSupervisors(ctx, segmentSupervisors, sourceValidUntil, effectiveDate)
	if err != nil {
		return nil, err
	}
	return &timetable.EndTemplateResult{
		TemplateID:        templateID,
		EffectiveDate:     effectiveDate,
		CappedSchedules:   cappedSchedules,
		CappedEnrollments: cappedEnrollments + cappedBoundedEnrollments,
		CappedSupervisors: cappedSupervisors + cappedBoundedSupervisors,
	}, nil
}

// cascadeEndToSeriesSegments caps every OTHER segment of the same split series
// whose window still reaches into [requestedFrom, ∞) and removes its planned
// occurrences (#2187). "Diesen und alle folgenden löschen" must leave no live
// occurrence at or after the chosen date anywhere in the lineage — and the
// grid renders occurrences with the id of the segment that materialized them,
// so the request can just as well arrive on a capped predecessor while the
// living successor carries the later occurrences. The cascade therefore runs
// over the whole lineage in both directions, the still-open segment included.
// capAt never inverts a window: a date before the segment start degrades the
// segment to an empty window (valid_until == valid_from), the same shape a
// same-day split leaves behind. Returns the planned instances deleted across
// all cascaded segments.
func (s *TemplateSplitService) cascadeEndToSeriesSegments(
	ctx context.Context,
	templateID int64,
	requestedFrom timezone.Date,
) (int64, error) {
	segments, err := loadTemplateSeriesSegments(ctx, s.deps.GroupRepo, s.deps.ScheduleRepo, s.getLogger(), templateID)
	if err != nil {
		return 0, err
	}
	var deleted int64
	for i := range segments {
		seg := &segments[i]
		if seg.Group == nil || seg.Group.ID == templateID {
			continue
		}
		capAt, ok := segmentCascadeCap(seg, requestedFrom)
		if !ok {
			continue // the segment already ends at or before the requested date
		}
		n, err := s.endCascadedSegment(ctx, templateID, seg, capAt)
		deleted += n
		if err != nil {
			return deleted, err
		}
	}
	return deleted, nil
}

// segmentCascadeCap returns where the cascade caps a segment: the requested
// date, but never before the segment start. A segment that already ends at or
// before that date is left alone.
func segmentCascadeCap(seg *templateSeriesSegment, requestedFrom timezone.Date) (timezone.Date, bool) {
	capAt := requestedFrom
	if seg.ValidFrom != nil && capAt.Before(*seg.ValidFrom) {
		capAt = *seg.ValidFrom
	}
	if seg.ValidUntil != nil && !capAt.Before(*seg.ValidUntil) {
		return timezone.Date(""), false
	}
	return capAt, true
}

// endCascadedSegment caps one cascaded segment and deletes its planned
// occurrences from capAt on.
func (s *TemplateSplitService) endCascadedSegment(
	ctx context.Context,
	templateID int64,
	seg *templateSeriesSegment,
	capAt timezone.Date,
) (int64, error) {
	enrollments, supervisors, err := s.loadSegmentRosterCandidates(ctx, seg.Group.ID, capAt, seg.ValidUntil)
	if err != nil {
		return 0, err
	}
	if err := s.capSplitSource(ctx, seg.Group.ID, capAt, enrollments, supervisors, seg.ValidUntil); err != nil {
		return 0, err
	}
	// Same guard the requested segment gets: ending a lineage a care
	// offering still depends on must fail, no matter which segment of it
	// the request came in on.
	if err := s.validateCareOfferingSeries(ctx, seg.Group.ID); err != nil {
		return 0, s.deps.CareOfferings.validationError(
			ctx,
			"end template: validate linked care offerings",
			"ending the recurrence is incompatible with an existing care offering",
			err,
		)
	}
	segmentID := seg.Group.ID
	n, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, scheduleModel.Date(capAt), nil, &segmentID, false)
	if err != nil {
		return 0, &ScheduleError{Op: "end template: delete cascaded segment planned instances", Err: err}
	}
	s.getLogger().Info("end cascade capped series segment",
		slog.Int64("template_id", templateID),
		slog.Int64("segment_id", segmentID),
		slog.String("cap_at", capAt.String()),
		slog.Int64("deleted_instances", n),
	)
	return n, nil
}
