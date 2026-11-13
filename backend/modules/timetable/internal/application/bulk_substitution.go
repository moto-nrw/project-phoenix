package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
	scheduleModel "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	date        calendar.Date
	absencePlan []deviationAbsenceOp
	subPlan     []deviationSubOp
	subs        []timetable.DeviationSubstitutionInput
}

// ApplyBulkSubstitution applies the whole multi-day save atomically. Runs
// inside the caller's tenant tx (TenantTxMiddleware).
func (s *staffDeviations) ApplyBulkSubstitution(ctx context.Context, in timetable.BulkSubstitutionInput) (*timetable.BulkSubstitutionResult, error) {
	dates, err := normalizeBulkDates(in.Dates, func() calendar.Date {
		return calendar.DateFromTime(s.now())
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

	plans, staff, err := s.planBulkDays(ctx, in, dates, reason)
	if err != nil {
		return nil, err
	}
	if err := s.lockDeviationStaff(ctx, staff); err != nil {
		return nil, err
	}
	return s.executeBulkPlans(ctx, in.ActorAccountID, plans)
}

// validateBulkStaff runs the request-level staff preconditions: positive ids,
// distinct absent/substitute, and existence of both referenced staff members.
func (s *staffDeviations) validateBulkStaff(ctx context.Context, in timetable.BulkSubstitutionInput) error {
	if in.AbsentStaffID <= 0 {
		return timetable.DeviationBadRequest("absent staff must be a positive id").WithCode(timetable.CodeDeviationSelectionInvalid).OnField("absent_staff_id")
	}
	if in.SubstituteStaffID != nil {
		if *in.SubstituteStaffID <= 0 {
			return timetable.DeviationBadRequest("substitute staff must be a positive id").WithCode(timetable.CodeDeviationSelectionInvalid).OnField("substitute_staff_id")
		}
		if *in.SubstituteStaffID == in.AbsentStaffID {
			return timetable.DeviationBadRequest("absent and substitute staff must differ").WithCode(timetable.CodeSubstituteSelf).OnField("substitute_staff_id")
		}
	}
	if _, err := s.findReferencedStaff(ctx, in.AbsentStaffID, "absent staff"); err != nil {
		return err
	}
	if in.SubstituteStaffID != nil {
		return s.ensureSubstituteStaff(ctx, *in.SubstituteStaffID)
	}
	return nil
}

// ensureSubstituteStaff requires an existing staff member who may replace
// others; an external caregiver never does (#4573 D6).
func (s *staffDeviations) ensureSubstituteStaff(ctx context.Context, staffID int64) error {
	staff, err := s.findReferencedStaff(ctx, staffID, "substitute staff")
	if err != nil {
		return err
	}
	if staff.HasExternalCaregiverRecord {
		return externalSubstituteRefusal("substitute_staff_id")
	}
	return nil
}

// findReferencedStaff maps a missing staff reference to the same 404 the
// single-day deviations save produces.
func (s *staffDeviations) findReferencedStaff(ctx context.Context, staffID int64, label string) (*timetable.StaffMember, error) {
	staff, err := s.deps.Staff.FindByID(ctx, staffID)
	if err != nil {
		if domain.IsRecordNotFound(err) {
			return nil, timetable.DeviationNotFound(fmt.Sprintf("%s not found", label)).WithCode(timetable.CodeStaffNotFound).OnField(staffField(label))
		}
		return nil, timetable.DeviationInternal("load staff failed", err)
	}
	if staff == nil || staff.ID == 0 {
		return nil, timetable.DeviationNotFound(fmt.Sprintf("%s not found", label)).WithCode(timetable.CodeStaffNotFound).OnField(staffField(label))
	}
	return staff, nil
}

// staffField names the request field of a staff reference label
// ("absent staff" → absent_staff_id).
func staffField(label string) string {
	return strings.ReplaceAll(label, " ", "_") + "_id"
}

// planBulkDays runs Phase A for every selected date under the already-held day
// locks: classify the day-wide absence or substitution without writing a row.
// A day where the person has no plannable assignments classifies to an empty
// plan and stays a no-op. The read sets of all dates load before the first
// classification (#5808); Phase A writes nothing, so every date still
// classifies against the rows read after all its locks.
func (s *staffDeviations) planBulkDays(ctx context.Context, in timetable.BulkSubstitutionInput, dates []calendar.Date, reason *string) ([]bulkDayPlan, []int64, error) {
	deviations := bulkDeviationInput(in, reason)
	readSets, err := s.loadBulkDeviationReadSets(ctx, deviations, dates)
	if err != nil {
		return nil, nil, err
	}
	plans := make([]bulkDayPlan, 0, len(dates))
	for i, date := range dates {
		plan, err := planBulkDay(in, deviations, date, readSets[i])
		if err != nil {
			return nil, nil, err
		}
		plans = append(plans, plan)
	}
	// Every date reads the same staff references.
	return plans, lockableDeviationStaff(deviations, readSets[0]), nil
}

// bulkDeviationInput is the single-day deviation input every selected date
// classifies: a day-wide absence, or a day-wide substitution.
func bulkDeviationInput(in timetable.BulkSubstitutionInput, reason *string) timetable.ApplyDeviationsInput {
	if in.SubstituteStaffID == nil {
		return timetable.ApplyDeviationsInput{Absences: []timetable.DeviationAbsenceInput{{
			StaffID: in.AbsentStaffID,
			Reason:  reason,
		}}}
	}
	return timetable.ApplyDeviationsInput{Substitutions: []timetable.DeviationSubstitutionInput{{
		AbsentStaffID:     in.AbsentStaffID,
		SubstituteStaffID: *in.SubstituteStaffID,
		Reason:            in.Reason,
	}}}
}

// loadBulkDeviationReadSets reads the Phase-A rows of every selected date.
// Each date keeps its own assignment read; the staff, instance and
// instance-staff reads run once for all dates. Every date's read set holds
// exactly the rows its single-date read would return, so a row whose
// instance is gone still fails that date's classification.
func (s *staffDeviations) loadBulkDeviationReadSets(ctx context.Context, in timetable.ApplyDeviationsInput, dates []calendar.Date) ([]*deviationReadSet, error) {
	staffIDs := deviationInputStaffIDs(in)
	staff, err := s.deps.Staff.FindByIDs(ctx, staffIDs)
	if err != nil {
		return nil, timetable.DeviationInternal("load staff failed", err)
	}
	rowsByDate := make([][]*scheduleModel.InstanceStaff, len(dates))
	instanceIDsByDate := make([][]int64, len(dates))
	allInstanceIDs := newOrderedIDs()
	for i, date := range dates {
		rows, err := assignmentsOfStaffMembersOn(ctx, s.deps.InstanceStaff, staffIDs, date)
		if err != nil {
			return nil, bulkDayError(date, timetable.DeviationInternal("load staff assignments failed", err))
		}
		rowsByDate[i] = rows
		instanceIDsByDate[i] = deviationInputInstanceIDs(0, in, rows)
		for _, id := range instanceIDsByDate[i] {
			allInstanceIDs.add(id)
		}
	}
	instances, err := instancesByID(ctx, s.deps.Instances.Reads, allInstanceIDs.ids)
	if err != nil {
		return nil, timetable.DeviationInternal("load target instances failed", err)
	}
	allRows, err := staffOfInstances(ctx, s.deps.InstanceStaff, allInstanceIDs.ids)
	if err != nil {
		return nil, timetable.DeviationInternal("load instance staff failed", err)
	}
	readSets := make([]*deviationReadSet, len(dates))
	for i := range dates {
		ids := make(map[int64]bool, len(instanceIDsByDate[i]))
		for _, id := range instanceIDsByDate[i] {
			ids[id] = true
		}
		dayInstances := slices.DeleteFunc(slices.Clone(instances), func(instance *scheduleModel.ActivityInstance) bool { return !ids[instance.ID] })
		dayRows := slices.DeleteFunc(slices.Clone(allRows), func(row *scheduleModel.InstanceStaff) bool { return !ids[row.InstanceID] })
		readSets[i] = newDeviationReadSet(staff, staffIDs, rowsByDate[i], dayInstances, dayRows)
	}
	if err := s.markSickStaff(ctx, dates, readSets); err != nil {
		return nil, err
	}
	return readSets, nil
}

func planBulkDay(in timetable.BulkSubstitutionInput, deviations timetable.ApplyDeviationsInput, date calendar.Date, readSet *deviationReadSet) (bulkDayPlan, error) {
	if in.SubstituteStaffID == nil {
		absencePlan, err := planAbsences(deviations.Absences, date, readSet)
		if err != nil {
			return bulkDayPlan{}, bulkDayError(date, err)
		}
		return bulkDayPlan{date: date, absencePlan: absencePlan}, nil
	}

	// The substitute must not already be absent in the DB on this date, nor
	// sick that day — the same day-wide rule validateDeviationStaff enforces
	// (#1840, #6292).
	if readSet.sickStaff[*in.SubstituteStaffID] {
		return bulkDayPlan{}, substituteSickOnDate(date)
	}
	for _, row := range readSet.rowsByStaff[*in.SubstituteStaffID] {
		if row.IsAbsent {
			return bulkDayPlan{}, timetable.DeviationBadRequest(fmt.Sprintf(
				"die Ersatzperson ist am %s selbst abwesend", date.Format("02.01.2006"))).
				WithCode(timetable.CodeSubstituteAbsentOnDate).OnField("substitute_staff_id").WithValues(timetable.RefusalValues{Date: date.Format(timetable.RefusalDateLayout)})
		}
	}

	subPlan, _, err := planSubstitutions(deviations.Substitutions, nil, nil, readSet)
	if err != nil {
		return bulkDayPlan{}, bulkDayError(date, err)
	}
	return bulkDayPlan{date: date, subPlan: subPlan, subs: deviations.Substitutions}, nil
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
func normalizeBulkDates(dates []calendar.Date, clocks ...func() calendar.Date) ([]calendar.Date, error) {
	if len(dates) == 0 {
		return nil, timetable.DeviationBadRequest("dates must not be empty").WithCode(timetable.CodeDatesRequired).OnField("dates")
	}
	today := calendar.TodayDate()
	if len(clocks) > 0 && clocks[0] != nil {
		today = clocks[0]()
	}
	seen := make(map[calendar.Date]bool, len(dates))
	out := make([]calendar.Date, 0, len(dates))
	for _, date := range dates {
		if date.Before(today) {
			return nil, timetable.DeviationBadRequest("dates must not be in the past").WithCode(timetable.CodeDatesInPast).OnField("dates").
				WithValues(timetable.RefusalValues{Date: date.Format(timetable.RefusalDateLayout)})
		}
		if seen[date] {
			continue
		}
		seen[date] = true
		out = append(out, date)
	}
	if len(out) > timetable.MaxBulkSubstitutionDates {
		return nil, timetable.DeviationBadRequest(fmt.Sprintf("at most %d dates per request", timetable.MaxBulkSubstitutionDates)).
			WithCode(timetable.CodeTooManyDates).OnField("dates").WithValues(timetable.RefusalValues{Max: timetable.MaxBulkSubstitutionDates})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out, nil
}

// bulkDayError prefixes a per-day classification error with the failing date
// so the all-or-nothing save tells the admin which day to deselect.
func bulkDayError(date calendar.Date, err error) error {
	var de *timetable.DeviationError
	if errors.As(err, &de) {
		details := de.Details
		details.Date = date.Format(timetable.RefusalDateLayout)
		return &timetable.DeviationError{
			Status:    de.Status,
			Code:      de.Code,
			ClientMsg: fmt.Sprintf("%s: %s", date.Format("02.01.2006"), de.ClientMsg),
			Cause:     de.Cause,
			Details:   details,
			Field:     de.Field,
		}
	}
	return err
}

// substituteSickOnDate refuses a substitute a sick report covers on date with
// the existing "absent on that day" code: a sick person is absent.
func substituteSickOnDate(date calendar.Date) error {
	return timetable.DeviationBadRequest(fmt.Sprintf(
		"die Ersatzperson ist am %s ganztägig abwesend", date.Format("02.01.2006"))).
		WithCode(timetable.CodeSubstituteAbsentOnDate).OnField("substitute_staff_id").WithValues(timetable.RefusalValues{Date: date.Format(timetable.RefusalDateLayout)})
}
