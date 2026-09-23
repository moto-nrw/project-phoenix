package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// replaceTemplateRoster rewrites the template's planned roster and returns
// the students whose enrollment rows the rewrite retired (deleted or closed)
// — the caller reconciles their already-materialized occurrences when the
// edit involved an offering source (#2147 review).
func (s *TemplateService) replaceTemplateRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	scheduleValidFrom, scheduleValidUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) ([]int64, error) {
	rosterValidFrom := in.RosterValidFrom
	if scheduleValidFrom != nil {
		rosterValidFrom = *scheduleValidFrom
	}
	if scheduleValidUntil != nil && rosterValidFrom.After(*scheduleValidUntil) {
		return nil, &ScheduleError{
			Op: "update template: replace roster",
			Err: fmt.Errorf(
				"roster valid_from %s is after segment valid_until %s",
				rosterValidFrom.String(),
				scheduleValidUntil.String(),
			),
		}
	}

	roster, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
	if err != nil {
		return nil, &ScheduleError{Op: "update template: resolve roster", Err: err}
	}

	protectedCoverage, retiredStudentIDs, err := s.retireTemplateEnrollments(
		ctx,
		in.TemplateID,
		in.CalendarPeriodID,
		in.Weekdays,
		rosterValidFrom,
		scheduleValidUntil,
		previousCalendarPeriodID,
	)
	if err != nil {
		return nil, err
	}
	// A child kept by a care-offering row is already on the roster with a
	// provenance this editor must not overwrite on the weekdays it covers.
	window := rosterWindow{from: rosterValidFrom, until: scheduleValidUntil, periodID: in.CalendarPeriodID}
	if err := s.createTemplateEnrollments(ctx, in.TemplateID, tenantID, window, excludeProtectedStudentWeekdays(roster.Students, protectedCoverage)); err != nil {
		return nil, err
	}

	if err := s.retireTemplateSupervisors(ctx, in.TemplateID, in.CalendarPeriodID, rosterValidFrom, scheduleValidUntil, previousCalendarPeriodID); err != nil {
		return nil, err
	}
	if err := s.createTemplateSupervisors(ctx, in.TemplateID, tenantID, window, roster.Staff); err != nil {
		return nil, err
	}
	return retiredStudentIDs, nil
}

// rosterWindow is the validity and calendar period the replaced roster rows
// are written with.
type rosterWindow struct {
	from     timezone.Date
	until    *timezone.Date
	periodID *int64
}

func (s *TemplateService) createTemplateEnrollments(ctx context.Context, templateID, tenantID int64, window rosterWindow, rows []resolvedRosterRow) error {
	for _, row := range rows {
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        row.PersonID,
			ActivityGroupID:  templateID,
			ValidFrom:        activitiesModel.Date(window.from),
			ValidUntil:       activityDatePtr(window.until),
			CalendarPeriodID: window.periodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		enrollment.SetTenantID(tenantID)
		if err := s.deps.StudentEnrollmentRepo.Create(ctx, enrollment); err != nil {
			return &ScheduleError{Op: "update template: create enrollment", Err: err}
		}
	}
	return nil
}

func (s *TemplateService) createTemplateSupervisors(ctx context.Context, templateID, tenantID int64, window rosterWindow, rows []resolvedRosterRow) error {
	for _, row := range rows {
		supervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          row.PersonID,
			GroupID:          templateID,
			IsPrimary:        row.IsPrimary,
			ValidFrom:        activitiesModel.Date(window.from),
			ValidUntil:       activityDatePtr(window.until),
			CalendarPeriodID: window.periodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		supervisor.SetTenantID(tenantID)
		if err := s.deps.ActivitySupervisorRepo.Create(ctx, supervisor); err != nil {
			return &ScheduleError{Op: "update template: create supervisor", Err: err}
		}
	}
	return nil
}

type rosterRetirementAction uint8

const (
	rosterRetirementSkip rosterRetirementAction = iota
	rosterRetirementPreserve
	rosterRetirementDelete
	rosterRetirementClose
)

// retireTemplateEnrollments retires the roster rows the replacement is about
// to rewrite. The second return value lists the students whose rows were
// actually deleted or closed (#2147 review) — their future coverage shrank,
// so already-materialized occurrences may need reconciling.
func (s *TemplateService) retireTemplateEnrollments(
	ctx context.Context,
	templateID int64,
	calendarPeriodID *int64,
	weekdays []int,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) (map[int64]protectedStudentCoverage, []int64, error) {
	rows, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "update template: load enrollments", Err: err}
	}
	protected, retiredStudentIDs, err := s.retireUnprotectedTemplateEnrollments(
		ctx,
		rows,
		calendarPeriodID,
		replacementFrom,
		replacementUntil,
		previousCalendarPeriodID,
	)
	if err != nil {
		return nil, nil, err
	}
	coverage, err := s.rebaseProtectedTemplateEnrollments(ctx, protected, calendarPeriodID, weekdays)
	if err != nil {
		return nil, nil, err
	}
	return coverage, retiredStudentIDs, nil
}

func (s *TemplateService) retireUnprotectedTemplateEnrollments(
	ctx context.Context,
	rows []*activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) ([]*activitiesModel.StudentEnrollment, []int64, error) {
	protected := make([]*activitiesModel.StudentEnrollment, 0)
	retiredStudentIDs := make([]int64, 0)
	retiredSeen := make(map[int64]bool)
	for _, row := range rows {
		if row != nil && enrollmentIsProtected(row) &&
			validityWindowsOverlap(timezone.Date(row.ValidFrom), timezoneDatePtr(row.ValidUntil), replacementFrom, replacementUntil) {
			protected = append(protected, row)
			continue
		}
		action := classifyEnrollmentRetirement(row, calendarPeriodID, replacementFrom, replacementUntil, previousCalendarPeriodID)
		if err := s.applyEnrollmentRetirement(ctx, row, action, replacementFrom); err != nil {
			return nil, nil, err
		}
		if (action == rosterRetirementDelete || action == rosterRetirementClose) && !retiredSeen[row.StudentID] {
			retiredSeen[row.StudentID] = true
			retiredStudentIDs = append(retiredStudentIDs, row.StudentID)
		}
	}
	return protected, retiredStudentIDs, nil
}

func (s *TemplateService) applyEnrollmentRetirement(
	ctx context.Context,
	row *activitiesModel.StudentEnrollment,
	action rosterRetirementAction,
	replacementFrom timezone.Date,
) error {
	switch action {
	case rosterRetirementDelete:
		if err := s.deps.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return &ScheduleError{Op: "update template: delete future enrollment", Err: err}
		}
	case rosterRetirementClose:
		if err := s.deps.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, activitiesModel.Date(replacementFrom)); err != nil {
			return &ScheduleError{Op: "update template: close enrollment", Err: err}
		}
	}
	return nil
}

func (s *TemplateService) rebaseProtectedTemplateEnrollments(
	ctx context.Context,
	protected []*activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	weekdays []int,
) (map[int64]protectedStudentCoverage, error) {
	if err := validateProtectedEnrollmentRebase(protected, calendarPeriodID); err != nil {
		return nil, &ScheduleError{Op: "update template: rebase protected enrollments", Err: err}
	}
	for _, row := range protected {
		if err := s.rebaseProtectedEnrollmentPeriod(ctx, row, calendarPeriodID); err != nil {
			return nil, err
		}
	}
	return buildProtectedStudentCoverage(
		protected,
		weekdays,
		func(row *activitiesModel.StudentEnrollment) bool {
			return rosterPeriodApplies(row.CalendarPeriodID, calendarPeriodID)
		},
	), nil
}

func (s *TemplateService) rebaseProtectedEnrollmentPeriod(
	ctx context.Context,
	row *activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
) error {
	if row.CalendarPeriodID == nil || calendarPeriodID == nil ||
		*row.CalendarPeriodID == *calendarPeriodID {
		return nil
	}
	row.CalendarPeriodID = cloneOptionalInt64(calendarPeriodID)
	if err := s.deps.StudentEnrollmentRepo.Update(ctx, row); err != nil {
		return &ScheduleError{Op: "update template: persist protected enrollment period", Err: err}
	}
	return nil
}

func classifyEnrollmentRetirement(
	row *activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) rosterRetirementAction {
	if row == nil || !validityWindowsOverlap(timezone.Date(row.ValidFrom), timezoneDatePtr(row.ValidUntil), replacementFrom, replacementUntil) {
		return rosterRetirementSkip
	}
	// Enrollment-offer rows and weekday-specific legacy rows are managed
	// outside this template editor. The request does not expose their
	// provenance or selected weekdays, so replacing them would silently
	// discard data and widen/narrow a care-offer assignment.
	if row.EnrollmentRequestChildID != nil || len(row.SelectedWeekdays) > 0 {
		if rosterPeriodApplies(row.CalendarPeriodID, calendarPeriodID) {
			return rosterRetirementPreserve
		}
		return rosterRetirementSkip
	}
	return classifyOwnedRosterRetirement(
		timezone.Date(row.ValidFrom),
		timezoneDatePtr(row.ValidUntil),
		ownedRosterPeriodMatches(row.CalendarPeriodID, calendarPeriodID, previousCalendarPeriodID),
		replacementFrom,
		replacementUntil,
	)
}

func (s *TemplateService) retireTemplateSupervisors(
	ctx context.Context,
	templateID int64,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) error {
	rows, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return &ScheduleError{Op: "update template: load supervisors", Err: err}
	}
	for _, row := range rows {
		switch classifySupervisorRetirement(row, calendarPeriodID, replacementFrom, replacementUntil, previousCalendarPeriodID) {
		case rosterRetirementDelete:
			if err := s.deps.ActivitySupervisorRepo.Delete(ctx, row.ID); err != nil {
				return &ScheduleError{Op: "update template: delete future supervisor", Err: err}
			}
		case rosterRetirementClose:
			if err := s.deps.ActivitySupervisorRepo.SetValidUntilByID(ctx, row.ID, activitiesModel.Date(replacementFrom)); err != nil {
				return &ScheduleError{Op: "update template: close supervisor", Err: err}
			}
		}
	}
	return nil
}

func classifySupervisorRetirement(
	row *activitiesModel.SupervisorPlanned,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) rosterRetirementAction {
	if row == nil || !validityWindowsOverlap(timezone.Date(row.ValidFrom), timezoneDatePtr(row.ValidUntil), replacementFrom, replacementUntil) {
		return rosterRetirementSkip
	}
	return classifyOwnedRosterRetirement(
		timezone.Date(row.ValidFrom),
		timezoneDatePtr(row.ValidUntil),
		ownedRosterPeriodMatches(row.CalendarPeriodID, calendarPeriodID, previousCalendarPeriodID),
		replacementFrom,
		replacementUntil,
	)
}

func ownedRosterPeriodMatches(rowPeriodID, targetPeriodID, previousPeriodID *int64) bool {
	return optionalInt64sEqual(rowPeriodID, targetPeriodID) ||
		optionalInt64sEqual(rowPeriodID, previousPeriodID)
}

func classifyOwnedRosterRetirement(
	validFrom timezone.Date,
	validUntil *timezone.Date,
	periodMatches bool,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) rosterRetirementAction {
	if !periodMatches {
		return rosterRetirementSkip
	}
	// Open rows are editor-managed. A bounded plain roster row is replaceable
	// only when it shares this segment's end; unrelated phase windows survive.
	if validUntil != nil && (replacementUntil == nil || *validUntil != *replacementUntil) {
		return rosterRetirementSkip
	}
	if validFrom.After(replacementFrom) {
		return rosterRetirementDelete
	}
	return rosterRetirementClose
}

func validityWindowsOverlap(
	leftFrom timezone.Date,
	leftUntil *timezone.Date,
	rightFrom timezone.Date,
	rightUntil *timezone.Date,
) bool {
	if leftUntil != nil && !rightFrom.Before(*leftUntil) {
		return false
	}
	if rightUntil != nil && !leftFrom.Before(*rightUntil) {
		return false
	}
	return true
}

func optionalInt64sEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// rosterPeriodApplies mirrors materialization semantics: an unscoped roster
// row applies to every selected period, while a scoped row applies only to the
// same period. A protected unscoped row therefore suppresses creation of a
// broad period-specific duplicate.
func rosterPeriodApplies(rowPeriodID, targetPeriodID *int64) bool {
	if rowPeriodID == nil {
		return true
	}
	return targetPeriodID != nil && *rowPeriodID == *targetPeriodID
}

func commonScheduleValidityEnvelope(schedules []*activitiesModel.Schedule) (*timezone.Date, *timezone.Date, error) {
	if len(schedules) == 0 {
		return nil, nil, nil
	}
	if schedules[0] == nil {
		return nil, nil, fmt.Errorf("%w: nil schedule row", timetable.ErrInconsistentTemplateScheduleValidity)
	}
	validFrom := cloneOptionalDate(timezoneDatePtr(schedules[0].ValidFrom))
	validUntil := cloneOptionalDate(timezoneDatePtr(schedules[0].ValidUntil))
	if validFrom != nil && validUntil != nil && validFrom.After(*validUntil) {
		return nil, nil, fmt.Errorf(
			"%w: segment valid_from %s is after valid_until %s",
			timetable.ErrInconsistentTemplateScheduleValidity,
			validFrom.String(),
			validUntil.String(),
		)
	}
	for _, schedule := range schedules[1:] {
		if schedule == nil {
			return nil, nil, fmt.Errorf("%w: nil schedule row", timetable.ErrInconsistentTemplateScheduleValidity)
		}
		if !optionalDatesEqual(validFrom, timezoneDatePtr(schedule.ValidFrom)) || !optionalDatesEqual(validUntil, timezoneDatePtr(schedule.ValidUntil)) {
			return nil, nil, fmt.Errorf(
				"%w: schedule %d does not match the segment envelope",
				timetable.ErrInconsistentTemplateScheduleValidity,
				schedule.ID,
			)
		}
	}
	return validFrom, validUntil, nil
}

func optionalDatesEqual(left, right *timezone.Date) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cloneOptionalDate(date *timezone.Date) *timezone.Date {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}

func activityDatePtr(date *timezone.Date) *activitiesModel.Date {
	if date == nil {
		return nil
	}
	value := activitiesModel.Date(*date)
	return &value
}

func timezoneDatePtr(date *activitiesModel.Date) *timezone.Date {
	if date == nil {
		return nil
	}
	value := timezone.Date(*date)
	return &value
}
