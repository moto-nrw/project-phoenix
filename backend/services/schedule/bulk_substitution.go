// Package schedule — Sammel-Vertretung across several days (#2284).
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
// in ascending order, sharing the total lock ordering of
// acquireSubstituteDayLocks so a bulk save never deadlocks against a re-plan
// window or a single-day deviation save.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MaxBulkSubstitutionDates caps one bulk save. 31 dates cover any realistic
// sick-leave window (the instances list endpoint caps its read window at 56
// days anyway); longer absences should be entered in slices.
const MaxBulkSubstitutionDates = 31

// BulkSubstitutionInput is the parsed Sammel-Vertretung request. A nil
// SubstituteStaffID marks the person absent on every selected day without
// assigning cover (the multi-day form of an absence-only deviation).
type BulkSubstitutionInput struct {
	AbsentStaffID     int64
	SubstituteStaffID *int64
	Dates             []timezone.Date
	Reason            *string
	ActorAccountID    *int64
}

// BulkSubstitutionDay is the per-day slice of the result: what the save
// touched on that date plus the substitute's time-overlap advisories.
type BulkSubstitutionDay struct {
	Date     timezone.Date
	Affected []DeviationAffected
	Warnings []SubstituteTimeConflict
}

// BulkSubstitutionResult is what ApplyBulkSubstitution returns on success.
// ActiveTouched and the counts drive the handler's SSE broadcast and log line,
// mirroring ApplyDeviationsResult.
type BulkSubstitutionResult struct {
	Days          []BulkSubstitutionDay
	ActiveTouched map[int64]*scheduleModel.ActivityInstance
	AppliedWrites int
	ClearedAcks   int
}

// bulkDayPlan is the fully-classified Phase-A result for one selected date:
// nothing here has written a row yet.
type bulkDayPlan struct {
	date        timezone.Date
	absencePlan []deviationAbsenceOp
	subPlan     []deviationSubOp
	subs        []DeviationSubstitutionInput
}

// ApplyBulkSubstitution applies the whole multi-day save atomically. Runs
// inside the caller's tenant tx (TenantTxMiddleware).
func (s *instanceService) ApplyBulkSubstitution(ctx context.Context, in BulkSubstitutionInput) (*BulkSubstitutionResult, error) {
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
	tenantID := tenant.FromContext(ctx)
	for _, date := range dates {
		if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenantID, date)); err != nil {
			return nil, devErrInternal("lock day failed", err)
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
func (s *instanceService) validateBulkStaff(ctx context.Context, in BulkSubstitutionInput) error {
	if in.AbsentStaffID <= 0 {
		return devErrBadRequest("absent staff must be a positive id")
	}
	if in.SubstituteStaffID != nil {
		if *in.SubstituteStaffID <= 0 {
			return devErrBadRequest("substitute staff must be a positive id")
		}
		if *in.SubstituteStaffID == in.AbsentStaffID {
			return devErrBadRequest("absent and substitute staff must differ")
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
func (s *instanceService) ensureStaffExists(ctx context.Context, staffID int64, label string) error {
	staff, err := s.deps.StaffRepo.FindByID(ctx, staffID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return devErrNotFound(fmt.Sprintf("%s not found", label))
		}
		return devErrInternal("load staff failed", err)
	}
	if staff == nil || staff.ID == 0 {
		return devErrNotFound(fmt.Sprintf("%s not found", label))
	}
	return nil
}

// planBulkDays runs Phase A for every selected date under the already-held day
// locks: classify the day-wide absence or substitution without writing a row.
// A day where the person has no plannable assignments classifies to an empty
// plan and stays a no-op.
func (s *instanceService) planBulkDays(ctx context.Context, in BulkSubstitutionInput, dates []timezone.Date, reason *string) ([]bulkDayPlan, error) {
	plans := make([]bulkDayPlan, 0, len(dates))
	for _, date := range dates {
		if in.SubstituteStaffID == nil {
			absences := []DeviationAbsenceInput{{
				StaffID: in.AbsentStaffID,
				Reason:  reason,
			}}
			readSet, err := s.loadDeviationReadSet(ctx, 0, ApplyDeviationsInput{Absences: absences}, date)
			if err != nil {
				return nil, bulkDayError(date, err)
			}
			absencePlan, err := planAbsences(absences, date, readSet)
			if err != nil {
				return nil, bulkDayError(date, err)
			}
			plans = append(plans, bulkDayPlan{date: date, absencePlan: absencePlan})
			continue
		}

		subs := []DeviationSubstitutionInput{{
			AbsentStaffID:     in.AbsentStaffID,
			SubstituteStaffID: *in.SubstituteStaffID,
			Reason:            in.Reason,
		}}
		readSet, err := s.loadDeviationReadSet(ctx, 0, ApplyDeviationsInput{Substitutions: subs}, date)
		if err != nil {
			return nil, bulkDayError(date, err)
		}
		// The substitute must not already be absent in the DB on this date —
		// the same day-wide rule validateDeviationStaff enforces (#1840).
		for _, row := range readSet.rowsByStaff[*in.SubstituteStaffID] {
			if row.IsAbsent {
				return nil, devErrBadRequest(fmt.Sprintf(
					"die Ersatzperson ist am %s selbst abwesend", date.Format("02.01.2006")))
			}
		}

		subPlan, _, err := planSubstitutions(subs, nil, nil, readSet)
		if err != nil {
			return nil, bulkDayError(date, err)
		}
		plans = append(plans, bulkDayPlan{date: date, subPlan: subPlan, subs: subs})
	}
	return plans, nil
}

// executeBulkPlans runs Phase B: every classified write, stale-ack clearing on
// covered blocks, and the per-day time-conflict advisories. Any failure here is
// a 5xx, which rolls the whole multi-day tx back.
func (s *instanceService) executeBulkPlans(ctx context.Context, actor *int64, plans []bulkDayPlan) (*BulkSubstitutionResult, error) {
	now := time.Now()
	result := &BulkSubstitutionResult{
		Days:          make([]BulkSubstitutionDay, 0, len(plans)),
		ActiveTouched: make(map[int64]*scheduleModel.ActivityInstance),
	}
	for _, plan := range plans {
		day := BulkSubstitutionDay{
			Date:     plan.date,
			Affected: []DeviationAffected{},
			Warnings: []SubstituteTimeConflict{},
		}
		for _, op := range plan.absencePlan {
			if err := s.ApplyAbsence(ctx, op.row, op.instance, op.reason, actor, result.ActiveTouched); err != nil {
				return nil, devErrInternal("mark absent failed", err)
			}
			day.Affected = append(day.Affected, affectedOf(op.instance, SubstituteActionMarkedAbsent))
		}
		for _, op := range plan.subPlan {
			if err := s.ApplySubstitute(ctx, op.write, op.subID, op.reason, now, actor, result.ActiveTouched); err != nil {
				return nil, devErrInternal("assign substitute failed", err)
			}
			day.Affected = append(day.Affected, affectedOf(op.write.Instance, op.write.Action))
		}

		// A block that was acknowledged as deliberately unstaffed and is now
		// covered again carries a stale ack — clear it, mirroring
		// reconcileOtherAcks on the single-day path (#1840).
		cleared := make(map[int64]bool)
		for _, op := range plan.subPlan {
			if !op.write.Instance.UnderstaffedAck || cleared[op.write.Instance.ID] {
				continue
			}
			if op.write.Action != SubstituteActionSubstituted && op.write.Action != SubstituteActionAlreadyOnInstance {
				continue
			}
			cleared[op.write.Instance.ID] = true
			if err := s.ClearUnderstaffedAckIfStaffed(ctx, op.write.Instance.ID, actor); err != nil {
				return nil, devErrInternal("clear stale understaffed ack failed", err)
			}
		}
		result.ClearedAcks += len(cleared)

		if len(plan.subs) > 0 {
			warnings, err := s.collectDeviationWarnings(ctx, plan.subs, plan.subPlan, plan.date)
			if err != nil {
				return nil, devErrInternal("bulk time-conflict detection failed", err)
			}
			day.Warnings = warnings
		}

		result.AppliedWrites += len(day.Affected)
		result.Days = append(result.Days, day)
	}
	return result, nil
}

// normalizeBulkDates validates, dedupes, and sorts the selected dates
// ascending (the lock-ordering requirement). Past dates are historical record,
// exactly like the single-day past-block guard.
func normalizeBulkDates(dates []timezone.Date, clocks ...func() timezone.Date) ([]timezone.Date, error) {
	if len(dates) == 0 {
		return nil, devErrBadRequest("dates must not be empty")
	}
	today := timezone.TodayDate()
	if len(clocks) > 0 && clocks[0] != nil {
		today = clocks[0]()
	}
	seen := make(map[timezone.Date]bool, len(dates))
	out := make([]timezone.Date, 0, len(dates))
	for _, date := range dates {
		if date.Before(today) {
			return nil, devErrBadRequest("dates must not be in the past")
		}
		if seen[date] {
			continue
		}
		seen[date] = true
		out = append(out, date)
	}
	if len(out) > MaxBulkSubstitutionDates {
		return nil, devErrBadRequest(fmt.Sprintf("at most %d dates per request", MaxBulkSubstitutionDates))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}

// bulkDayError prefixes a per-day classification error with the failing date
// so the all-or-nothing save tells the admin which day to deselect.
func bulkDayError(date timezone.Date, err error) error {
	var de *DeviationError
	if errors.As(err, &de) {
		return &DeviationError{
			Status:    de.Status,
			Code:      de.Code,
			ClientMsg: fmt.Sprintf("%s: %s", date.Format("02.01.2006"), de.ClientMsg),
			Cause:     de.Cause,
		}
	}
	return err
}
