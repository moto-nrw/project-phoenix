package compose

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Sammel-Vertretung across several days (#2284).
//
// ApplyBulkSubstitution applies ONE person's day-wide absence — optionally
// covered by ONE substitute — to a set of selected dates in a single atomic
// save. It is the multi-day sibling of ApplyDeviations: the per-day semantics
// (day-wide absence, substitute classification, understaffed-ack
// reconciliation, time-conflict advisories) are exactly the single-day rules,
// reusing the same plan/classify/write helpers so the two paths cannot
// diverge.
//
// Atomicity mirrors deviation_apply.go: TenantTxMiddleware rolls the request
// tx back only on 5xx, so Phase A validates and classifies EVERY selected day
// before Phase B writes a single row — a 4xx raised mid-write would otherwise
// commit a partial multi-day save. Day locks are taken for all selected dates
// in ascending order, sharing the total lock ordering of the re-plan window
// locks so a bulk save never deadlocks against a re-plan window or a
// single-day deviation save.

// bulkDayPlan is the fully-classified Phase-A result for one selected date:
// nothing here has written a row yet.
type bulkDayPlan struct {
	date        timezone.Date
	absencePlan []deviationAbsenceOp
	subPlan     []deviationSubOp
	subs        []timetable.DeviationSubstitutionInput
}

// ApplyBulkSubstitution applies the whole multi-day save atomically. Runs
// inside the caller's tenant tx (TenantTxMiddleware).
func (s *staffDeviations) ApplyBulkSubstitution(ctx context.Context, in timetable.BulkSubstitutionInput) (*timetable.BulkSubstitutionResult, error) {
	dates, err := normalizeBulkDates(in.Dates, func() timezone.Date {
		return timezone.DateFromTime(s.now())
	})
	if err != nil {
		return nil, err
	}
	if err := s.validateBulkStaff(ctx, in); err != nil {
		return nil, err
	}
	reason := trimDeviationReason(in.Reason)

	// Ascending-order day locks BEFORE any classification read — the same
	// total ordering every other day-wide staffing mutation uses (#1840).
	for _, date := range dates {
		if err := s.acquireSubstituteDayLock(ctx, date); err != nil {
			return nil, timetable.DeviationInternal("lock day failed", err)
		}
	}

	plans, err := s.planBulkDays(ctx, in, dates, reason)
	if err != nil {
		return nil, err
	}
	return s.executeBulkPlans(ctx, in.ActorAccountID, plans)
}

// validateBulkStaff runs the request-level staff preconditions: positive ids,
// distinct absent/substitute, and existence of both referenced staff members.
func (s *staffDeviations) validateBulkStaff(ctx context.Context, in timetable.BulkSubstitutionInput) error {
	if in.AbsentStaffID <= 0 {
		return timetable.DeviationBadRequest("absent staff must be a positive id")
	}
	if in.SubstituteStaffID != nil {
		if *in.SubstituteStaffID <= 0 {
			return timetable.DeviationBadRequest("substitute staff must be a positive id")
		}
		if *in.SubstituteStaffID == in.AbsentStaffID {
			return timetable.DeviationBadRequest("absent and substitute staff must differ")
		}
	}
	if err := s.ensureStaffExists(ctx, in.AbsentStaffID, "absent staff"); err != nil {
		return err
	}
	if in.SubstituteStaffID != nil {
		if err := s.ensureStaffExists(ctx, *in.SubstituteStaffID, "substitute staff"); err != nil {
			return err
		}
	}
	return nil
}

// ensureStaffExists maps a missing staff reference to the same 404 the
// single-day deviations save produces.
func (s *staffDeviations) ensureStaffExists(ctx context.Context, staffID int64, label string) error {
	staff, err := s.deps.Staff.FindByID(ctx, staffID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return timetable.DeviationNotFound(fmt.Sprintf("%s not found", label))
		}
		return timetable.DeviationInternal("load staff failed", err)
	}
	if staff == nil || staff.ID == 0 {
		return timetable.DeviationNotFound(fmt.Sprintf("%s not found", label))
	}
	return nil
}

// planBulkDays runs Phase A for every selected date under the already-held day
// locks: classify the day-wide absence or substitution without writing a row.
// A day where the person has no plannable assignments classifies to an empty
// plan and stays a no-op.
func (s *staffDeviations) planBulkDays(ctx context.Context, in timetable.BulkSubstitutionInput, dates []timezone.Date, reason *string) ([]bulkDayPlan, error) {
	plans := make([]bulkDayPlan, 0, len(dates))
	for _, date := range dates {
		plan, err := s.planBulkDay(ctx, in, date, reason)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s *staffDeviations) planBulkDay(ctx context.Context, in timetable.BulkSubstitutionInput, date timezone.Date, reason *string) (bulkDayPlan, error) {
	if in.SubstituteStaffID == nil {
		absences := []timetable.DeviationAbsenceInput{{
			StaffID: in.AbsentStaffID,
			Reason:  reason,
		}}
		readSet, err := s.loadDeviationReadSet(ctx, 0, timetable.ApplyDeviationsInput{Absences: absences}, date)
		if err != nil {
			return bulkDayPlan{}, bulkDayError(date, err)
		}
		absencePlan, err := planAbsences(absences, date, readSet)
		if err != nil {
			return bulkDayPlan{}, bulkDayError(date, err)
		}
		return bulkDayPlan{date: date, absencePlan: absencePlan}, nil
	}

	subs := []timetable.DeviationSubstitutionInput{{
		AbsentStaffID:     in.AbsentStaffID,
		SubstituteStaffID: *in.SubstituteStaffID,
		Reason:            in.Reason,
	}}
	readSet, err := s.loadDeviationReadSet(ctx, 0, timetable.ApplyDeviationsInput{Substitutions: subs}, date)
	if err != nil {
		return bulkDayPlan{}, bulkDayError(date, err)
	}
	// The substitute must not already be absent in the DB on this date —
	// the same day-wide rule validateDeviationStaff enforces (#1840).
	for _, row := range readSet.rowsByStaff[*in.SubstituteStaffID] {
		if row.IsAbsent {
			return bulkDayPlan{}, timetable.DeviationBadRequest(fmt.Sprintf(
				"die Ersatzperson ist am %s selbst abwesend", date.Format("02.01.2006")))
		}
	}

	subPlan, _, err := planSubstitutions(subs, nil, nil, readSet)
	if err != nil {
		return bulkDayPlan{}, bulkDayError(date, err)
	}
	return bulkDayPlan{date: date, subPlan: subPlan, subs: subs}, nil
}

// executeBulkPlans runs Phase B: every classified write, stale-ack clearing on
// covered blocks, and the per-day time-conflict advisories. Any failure here is
// a 5xx, which rolls the whole multi-day tx back.
func (s *staffDeviations) executeBulkPlans(ctx context.Context, actor *int64, plans []bulkDayPlan) (*timetable.BulkSubstitutionResult, error) {
	now := time.Now()
	result := &timetable.BulkSubstitutionResult{
		Days:          make([]timetable.BulkSubstitutionDay, 0, len(plans)),
		ActiveTouched: make(timetable.TouchedActivities),
	}
	for _, plan := range plans {
		day, cleared, err := s.executeBulkDay(ctx, actor, plan, now, result.ActiveTouched)
		if err != nil {
			return nil, err
		}
		result.ClearedAcks += cleared
		result.AppliedWrites += len(day.Affected)
		result.Days = append(result.Days, day)
	}
	return result, nil
}

func (s *staffDeviations) executeBulkDay(ctx context.Context, actor *int64, plan bulkDayPlan, now time.Time, touched timetable.TouchedActivities) (timetable.BulkSubstitutionDay, int, error) {
	day := timetable.BulkSubstitutionDay{
		Date:     plan.date,
		Affected: []timetable.DeviationAffected{},
		Warnings: []timetable.SubstituteTimeConflict{},
	}
	for _, op := range plan.absencePlan {
		if err := s.applyAbsence(ctx, op.row, op.instance, op.reason, actor, touched); err != nil {
			return day, 0, timetable.DeviationInternal("mark absent failed", err)
		}
		day.Affected = append(day.Affected, affectedOf(op.instance, timetable.SubstituteActionMarkedAbsent))
	}
	for _, op := range plan.subPlan {
		if err := s.applySubstitute(ctx, op.write, op.subID, op.reason, now, actor, touched); err != nil {
			return day, 0, timetable.DeviationInternal("assign substitute failed", err)
		}
		day.Affected = append(day.Affected, affectedOf(op.write.Instance, op.write.Action))
	}

	cleared, err := s.clearCoveredAcks(ctx, actor, plan.subPlan)
	if err != nil {
		return day, 0, err
	}

	if len(plan.subs) > 0 {
		warnings, err := s.collectDeviationWarnings(ctx, plan.subs, plan.subPlan, plan.date)
		if err != nil {
			return day, 0, timetable.DeviationInternal("bulk time-conflict detection failed", err)
		}
		day.Warnings = warnings
	}
	return day, cleared, nil
}

// clearCoveredAcks clears the stale acknowledgement of a block that was
// acknowledged as deliberately unstaffed and is now covered again, mirroring
// reconcileOtherAcks on the single-day path (#1840).
func (s *staffDeviations) clearCoveredAcks(ctx context.Context, actor *int64, subPlan []deviationSubOp) (int, error) {
	cleared := make(map[int64]bool)
	for _, op := range subPlan {
		if cleared[op.write.Instance.ID] || !coversAcknowledgedBlock(op) {
			continue
		}
		cleared[op.write.Instance.ID] = true
		if err := s.deps.Lifecycle.ClearUnderstaffedAckIfStaffed(ctx, op.write.Instance.ID, actor); err != nil {
			return 0, timetable.DeviationInternal("clear stale understaffed ack failed", err)
		}
	}
	return len(cleared), nil
}

// normalizeBulkDates validates, dedupes, and sorts the selected dates
// ascending (the lock-ordering requirement). Past dates are historical record,
// exactly like the single-day past-block guard.
func normalizeBulkDates(dates []timezone.Date, clocks ...func() timezone.Date) ([]timezone.Date, error) {
	if len(dates) == 0 {
		return nil, timetable.DeviationBadRequest("dates must not be empty")
	}
	today := timezone.TodayDate()
	if len(clocks) > 0 && clocks[0] != nil {
		today = clocks[0]()
	}
	seen := make(map[timezone.Date]bool, len(dates))
	out := make([]timezone.Date, 0, len(dates))
	for _, date := range dates {
		if date.Before(today) {
			return nil, timetable.DeviationBadRequest("dates must not be in the past")
		}
		if seen[date] {
			continue
		}
		seen[date] = true
		out = append(out, date)
	}
	if len(out) > timetable.MaxBulkSubstitutionDates {
		return nil, timetable.DeviationBadRequest(fmt.Sprintf("at most %d dates per request", timetable.MaxBulkSubstitutionDates))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}

// bulkDayError prefixes a per-day classification error with the failing date
// so the all-or-nothing save tells the admin which day to deselect.
func bulkDayError(date timezone.Date, err error) error {
	var de *timetable.DeviationError
	if errors.As(err, &de) {
		return &timetable.DeviationError{
			Status:    de.Status,
			Code:      de.Code,
			ClientMsg: fmt.Sprintf("%s: %s", date.Format("02.01.2006"), de.ClientMsg),
			Cause:     de.Cause,
		}
	}
	return err
}
