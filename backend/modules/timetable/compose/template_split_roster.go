package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// loadSegmentRosterCandidates returns editor-owned enrollment and supervisor
// rows whose validity overlaps the segment tail. This includes future-starting
// open rows (their start is preserved) and plain bounded rows whose end equals
// the source segment end when a predecessor is split a second time. Sourced,
// weekday-specific, and unrelated bounded windows stay with their owning
// workflow. Since migration 1.15.52 the partial
// unique indexes are period-scoped — (tenant, person, group,
// COALESCE(calendar_period_id, 0)) WHERE valid_until IS NULL — so a person
// can have SEVERAL active rows on the same group, one per calendar period.
// Carry-over must therefore dedupe per person before stamping the successor's
// calendar_period_id, or the insert violates the index (see
// createStudentRoster / createStaffRoster).
func (s *TemplateSplitService) loadSegmentRosterCandidates(
	ctx context.Context,
	groupID int64,
	effectiveDate timezone.Date,
	segmentValidUntil *timezone.Date,
) ([]*activitiesModel.StudentEnrollment, []*activitiesModel.SupervisorPlanned, error) {
	enrollments, err := s.deps.EnrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: load enrollments", Err: err}
	}
	supervisors, err := s.deps.SupervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: load supervisors", Err: err}
	}

	activeEnrollments := make([]*activitiesModel.StudentEnrollment, 0, len(enrollments))
	for _, e := range enrollments {
		if e != nil && validityWindowsOverlap(timezone.Date(e.ValidFrom), timezoneDatePtr(e.ValidUntil), effectiveDate, segmentValidUntil) &&
			(enrollmentIsProtected(e) || enrollmentBelongsToSegmentRoster(e, segmentValidUntil)) {
			activeEnrollments = append(activeEnrollments, e)
		}
	}
	activeSupervisors := make([]*activitiesModel.SupervisorPlanned, 0, len(supervisors))
	for _, sp := range supervisors {
		if sp != nil && validityWindowsOverlap(timezone.Date(sp.ValidFrom), timezoneDatePtr(sp.ValidUntil), effectiveDate, segmentValidUntil) &&
			supervisorBelongsToSegmentRoster(sp, segmentValidUntil) {
			activeSupervisors = append(activeSupervisors, sp)
		}
	}
	return activeEnrollments, activeSupervisors, nil
}

func enrollmentBelongsToSegmentRoster(row *activitiesModel.StudentEnrollment, segmentValidUntil *timezone.Date) bool {
	if enrollmentIsProtected(row) {
		return false
	}
	if row.ValidUntil == nil {
		return true
	}
	return optionalDatesEqual(timezoneDatePtr(row.ValidUntil), segmentValidUntil)
}

func enrollmentIsProtected(row *activitiesModel.StudentEnrollment) bool {
	return row.EnrollmentRequestChildID != nil || len(row.SelectedWeekdays) > 0
}

func supervisorBelongsToSegmentRoster(row *activitiesModel.SupervisorPlanned, segmentValidUntil *timezone.Date) bool {
	return row.ValidUntil == nil || optionalDatesEqual(timezoneDatePtr(row.ValidUntil), segmentValidUntil)
}

func (s *TemplateSplitService) capBoundedSegmentEnrollments(
	ctx context.Context,
	rows []*activitiesModel.StudentEnrollment,
	segmentValidUntil *timezone.Date,
	capAt timezone.Date,
) (int64, error) {
	if segmentValidUntil == nil {
		return 0, nil
	}
	var changed int64
	for _, row := range rows {
		if enrollmentIsProtected(row) || row.ValidUntil == nil || !optionalDatesEqual(timezoneDatePtr(row.ValidUntil), segmentValidUntil) {
			continue
		}
		if !row.ValidFrom.Before(capAt) {
			if err := s.deps.EnrollmentRepo.Delete(ctx, row.ID); err != nil {
				return changed, &ScheduleError{Op: "cap segment: delete future enrollment", Err: err}
			}
		} else if err := s.deps.EnrollmentRepo.SetValidUntilByID(ctx, row.ID, activitiesModel.Date(capAt)); err != nil {
			return changed, &ScheduleError{Op: "cap segment: close enrollment", Err: err}
		}
		changed++
	}
	return changed, nil
}

func (s *TemplateSplitService) capBoundedSegmentSupervisors(
	ctx context.Context,
	rows []*activitiesModel.SupervisorPlanned,
	segmentValidUntil *timezone.Date,
	capAt timezone.Date,
) (int64, error) {
	if segmentValidUntil == nil {
		return 0, nil
	}
	var changed int64
	for _, row := range rows {
		if row.ValidUntil == nil || !optionalDatesEqual(timezoneDatePtr(row.ValidUntil), segmentValidUntil) {
			continue
		}
		if !row.ValidFrom.Before(capAt) {
			if err := s.deps.SupervisorRepo.Delete(ctx, row.ID); err != nil {
				return changed, &ScheduleError{Op: "cap segment: delete future supervisor", Err: err}
			}
		} else if err := s.deps.SupervisorRepo.SetValidUntilByID(ctx, row.ID, activitiesModel.Date(capAt)); err != nil {
			return changed, &ScheduleError{Op: "cap segment: close supervisor", Err: err}
		}
		changed++
	}
	return changed, nil
}

// createSuccessorGroup copies the old template and applies the updated fields.
func (s *TemplateSplitService) createSuccessorGroup(ctx context.Context, old *activitiesModel.Group, in TemplateSplitInput, tenantID int64) (*activitiesModel.Group, error) {
	group := successorGroup(old, in)
	group.SetTenantID(tenantID)
	if err := s.deps.GroupRepo.Create(ctx, group); err != nil {
		return nil, &ScheduleError{Op: "split template: create successor template", Err: err}
	}
	targetRepo, ok := s.deps.GroupRepo.(activitiesModel.GroupTargetRepository)
	if !ok && len(in.Targets) > 0 {
		return nil, &ScheduleError{Op: "split template: create successor targets", Err: errors.New("target repository is not configured")}
	}
	if ok {
		if err := targetRepo.ReplaceTargets(ctx, group.ID, in.Targets); err != nil {
			return nil, &ScheduleError{Op: "split template: create successor targets", Err: err}
		}
	}
	return group, nil
}

// successorGroup builds the successor template row. The presence-aware
// fields follow the same omitted-vs-null rule: an omitted field inherits the
// old template's value, a provided one is authoritative and a provided null
// clears it — the Personalbedarf override (#1839), the Wochennotiz (#1837
// follow-up), the Listenart (#1565) and the planning track. MaxParticipants
// falls back to the old template's value when not provided.
func successorGroup(old *activitiesModel.Group, in TemplateSplitInput) *activitiesModel.Group {
	seriesRootID := old.ID
	if old.SeriesRootID != nil {
		seriesRootID = *old.SeriesRootID
	}
	sourceCareOfferingIDs, sourceGradeLevels, sourceSchoolClasses := successorOfferingSource(in)
	roomID := in.RoomID
	return &activitiesModel.Group{
		Name:                  in.Name,
		MaxParticipants:       successorMaxParticipants(old, in),
		RequiredStaff:         providedOrInherited(in.RequiredStaffProvided, in.RequiredStaff, old.RequiredStaff),
		IsOpen:                old.IsOpen,
		CategoryID:            in.CategoryID,
		PlanningTrackID:       providedOrInherited(in.PlanningTrackIDProvided, in.PlanningTrackID, old.PlanningTrackID),
		PlannedRoomID:         &roomID,
		CreatedBy:             old.CreatedBy,
		Type:                  in.Type,
		EducationGroupID:      in.EducationGroupID,
		IsTemplate:            true,
		SeriesRootID:          &seriesRootID,
		CalendarPeriodID:      in.CalendarPeriodID,
		TargetGroupType:       in.TargetGroupType,
		TargetGradeLevel:      in.TargetGradeLevel,
		TargetSchoolClass:     in.TargetSchoolClass,
		SourceCareOfferingIDs: sourceCareOfferingIDs,
		SourceGradeLevels:     sourceGradeLevels,
		SourceSchoolClasses:   sourceSchoolClasses,
		ListKind:              providedOrInherited(in.ListKindProvided, in.ListKind, old.ListKind),
		Notes:                 providedOrInherited(in.NotesProvided, in.Notes, old.Notes),
		IncludeClosingDays:    successorIncludesClosingDays(in.IncludeClosingDays, old.IncludeClosingDays),
		SeriesLastDay:         old.SeriesLastDay,
	}
}

// successorIncludesClosingDays resolves the closing-day opt-in a series split
// carries to its successor (#3594): an explicit request value wins, an
// omitted one inherits the source series' flag.
func successorIncludesClosingDays(requested *bool, inherited bool) bool {
	if requested != nil {
		return *requested
	}
	return inherited
}

func successorMaxParticipants(old *activitiesModel.Group, in TemplateSplitInput) int {
	if !in.MaxParticipantsProvided && in.MaxParticipants == nil {
		return old.MaxParticipants
	}
	if in.MaxParticipants == nil {
		return 0
	}
	return *in.MaxParticipants
}

// providedOrInherited returns the request's value when the request carried
// the field, else the inherited one.
func providedOrInherited[T any](provided bool, requested, inherited *T) *T {
	if provided {
		return requested
	}
	return inherited
}

// successorOfferingSource is the merged offering-source rule (#2137):
// resolveSuccessorOfferingSource already merged the presence-aware request
// fields with the old template (omitted → inherit, provided → authoritative,
// dropped on a Zielgruppe change away from 'angebot') and validated the
// result — the successor persists the merged rule verbatim (#2147 review
// round 14). A filter only travels with a source, and the class filter only
// without a grade filter.
func successorOfferingSource(in TemplateSplitInput) ([]int64, []int, []string) {
	sourceCareOfferingIDs := append([]int64(nil), in.SourceCareOfferingIDs...)
	var sourceGradeLevels []int
	if len(sourceCareOfferingIDs) > 0 && len(in.SourceGradeLevels) > 0 {
		sourceGradeLevels = append(sourceGradeLevels, in.SourceGradeLevels...)
	}
	var sourceSchoolClasses []string
	if len(sourceCareOfferingIDs) > 0 && len(sourceGradeLevels) == 0 && len(in.SourceSchoolClasses) > 0 {
		sourceSchoolClasses = append(sourceSchoolClasses, in.SourceSchoolClasses...)
	}
	return sourceCareOfferingIDs, sourceGradeLevels, sourceSchoolClasses
}

// createSuccessorSchedules creates one schedule row per weekday, starting at
// the effective date (valid_from inclusive) with an open end (valid_until
// NULL).
func (s *TemplateSplitService) createSuccessorSchedules(
	ctx context.Context,
	groupID, timeframeID int64,
	in TemplateSplitInput,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) ([]int64, error) {
	weekPattern := 0
	if in.WeekPattern != nil {
		weekPattern = *in.WeekPattern
	}
	scheduleIDs := make([]int64, 0, len(in.Weekdays))
	for _, weekday := range in.Weekdays {
		tfID := timeframeID
		validFrom := activitiesModel.Date(in.EffectiveDate)
		sched := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &tfID,
			ActivityGroupID:  groupID,
			WeekPattern:      weekPattern,
			CalendarPeriodID: in.CalendarPeriodID,
			ValidFrom:        &validFrom, // never materialize before the split point
			ValidUntil:       activityDatePtr(segmentValidUntil),
		}
		sched.SetTenantID(tenantID)
		if err := s.deps.ScheduleRepo.Create(ctx, sched); err != nil {
			return nil, &ScheduleError{Op: "split template: create successor schedule", Err: err}
		}
		scheduleIDs = append(scheduleIDs, sched.ID)
	}
	return scheduleIDs, nil
}

// createStudentRoster writes the successor's enrollments. Explicit StudentIDs
// replace the editor-owned manual roster; sourced or weekday-specific rows are
// carried independently because the editor cannot represent their provenance
// or per-weekday selection. Nil carries the manual roster too. Explicit rows
// start at the effective date; carried rows retain a later valid_from. Every
// row is clipped to the source segment's upper bound.
//
// The carry path dedupes per student: loadActiveRoster can return several
// active rows per student (one per calendar period since migration 1.15.52),
// and stamping them all with in.CalendarPeriodID would collide on
// idx_student_enrollments_active. The row matching in.CalendarPeriodID is
// preferred; selected_weekdays are unioned across the student's rows (an
// empty list means "all weekdays" and wins the union).
func (s *TemplateSplitService) createStudentRoster(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	carried []*activitiesModel.StudentEnrollment,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	manual, protected := partitionCarriedEnrollments(carried)
	if err := validateProtectedEnrollmentRebase(protected, in.CalendarPeriodID); err != nil {
		return &ScheduleError{Op: "split template: rebase protected enrollments", Err: err}
	}
	protectedCoverage := buildProtectedStudentCoverage(
		protected,
		in.Weekdays,
		func(row *activitiesModel.StudentEnrollment) bool {
			return rosterPeriodApplies(row.CalendarPeriodID, in.CalendarPeriodID) ||
				(in.CalendarPeriodID != nil && row.CalendarPeriodID != nil)
		},
	)
	rows := buildSuccessorStudentRows(in, manual, protectedCoverage, segmentValidUntil)
	if err := s.persistSuccessorStudentRows(ctx, groupID, in, rows, tenantID, segmentValidUntil); err != nil {
		return err
	}
	return s.persistProtectedStudentRows(ctx, groupID, in, protected, tenantID, segmentValidUntil)
}

func partitionCarriedEnrollments(
	carried []*activitiesModel.StudentEnrollment,
) (manual, protected []*activitiesModel.StudentEnrollment) {
	manual = make([]*activitiesModel.StudentEnrollment, 0, len(carried))
	protected = make([]*activitiesModel.StudentEnrollment, 0, len(carried))
	for _, row := range carried {
		if enrollmentIsProtected(row) {
			protected = append(protected, row)
		} else {
			manual = append(manual, row)
		}
	}
	return manual, protected
}

func validateProtectedEnrollmentRebase(
	rows []*activitiesModel.StudentEnrollment,
	targetPeriodID *int64,
) error {
	if targetPeriodID == nil {
		return nil
	}
	activeScopedByStudent := make(map[int64]int)
	for _, row := range rows {
		if row == nil || row.CalendarPeriodID == nil || row.ValidUntil != nil {
			continue
		}
		activeScopedByStudent[row.StudentID]++
		if activeScopedByStudent[row.StudentID] > 1 {
			return fmt.Errorf(
				"%w: student %d has multiple active protected period rows",
				timetable.ErrTemplateRosterRebaseConflict,
				row.StudentID,
			)
		}
	}
	return nil
}

func buildSuccessorStudentRows(
	in TemplateSplitInput,
	carried []*activitiesModel.StudentEnrollment,
	protectedCoverage map[int64]protectedStudentCoverage,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.StudentEnrollment {
	if in.StudentIDs != nil {
		resolved, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, nil, nil, in.WeekdayAssignments)
		if err != nil {
			resolved = resolvedTemplateRoster{Students: sharedRosterRows(in.StudentIDs, nil)}
		}
		manualRows := excludeProtectedStudentWeekdays(resolved.Students, protectedCoverage)
		rows := make([]*activitiesModel.StudentEnrollment, 0, len(manualRows))
		for _, row := range manualRows {
			rows = append(rows, &activitiesModel.StudentEnrollment{
				StudentID: row.PersonID,
				Weekday:   weekdayScopePtr(row.Weekday),
			})
		}
		return rows
	}
	return carriedStudentRowsByPerson(carried, in.CalendarPeriodID, segmentValidUntil)
}

func carriedStudentRowsByPerson(
	carried []*activitiesModel.StudentEnrollment,
	periodID *int64,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.StudentEnrollment {
	// Keyed by (student, weekday scope) — see carriedStaffRowsByPerson for why
	// the weekday must be part of the key (#2129).
	byStudent := make(map[rosterPersonScope][]*activitiesModel.StudentEnrollment, len(carried))
	order := make([]rosterPersonScope, 0, len(carried))
	for _, row := range carried {
		key := rosterPersonScope{PersonID: row.StudentID, Weekday: weekdayScopeKey(row.Weekday)}
		if _, seen := byStudent[key]; !seen {
			order = append(order, key)
		}
		byStudent[key] = append(byStudent[key], row)
	}
	rows := make([]*activitiesModel.StudentEnrollment, 0, len(order))
	for _, key := range order {
		group := byStudent[key]
		preferred := preferCarriedRow(group, periodID, func(row *activitiesModel.StudentEnrollment) *int64 { return row.CalendarPeriodID })
		rows = append(rows, &activitiesModel.StudentEnrollment{
			StudentID:        key.PersonID,
			ValidFrom:        preferred.ValidFrom,
			ValidUntil:       earliestActivityDate(preferred.ValidUntil, segmentValidUntil),
			SelectedWeekdays: unionSelectedWeekdays(group, preferred),
			Weekday:          weekdayScopePtr(preferred.Weekday),
		})
	}
	return rows
}

func (s *TemplateSplitService) persistSuccessorStudentRows(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	rows []*activitiesModel.StudentEnrollment,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	for _, row := range rows {
		row.ActivityGroupID = groupID
		if row.ValidFrom.IsZero() || row.ValidFrom.Before(in.EffectiveDate) {
			row.ValidFrom = activitiesModel.Date(in.EffectiveDate)
		}
		row.ValidUntil = earliestActivityDate(row.ValidUntil, segmentValidUntil)
		row.CalendarPeriodID = in.CalendarPeriodID
		row.SetTenantID(tenantID)
		if err := s.deps.EnrollmentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: create enrollment", Err: err}
		}
	}
	return nil
}

func (s *TemplateSplitService) persistProtectedStudentRows(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	rows []*activitiesModel.StudentEnrollment,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	for _, source := range rows {
		validFrom := source.ValidFrom
		if validFrom.Before(in.EffectiveDate) {
			validFrom = activitiesModel.Date(in.EffectiveDate)
		}
		row := &activitiesModel.StudentEnrollment{
			StudentID:                source.StudentID,
			ActivityGroupID:          groupID,
			ValidFrom:                validFrom,
			ValidUntil:               earliestActivityDate(source.ValidUntil, segmentValidUntil),
			CalendarPeriodID:         protectedEnrollmentPeriodAfterRebase(source.CalendarPeriodID, in.CalendarPeriodID),
			EnrollmentRequestChildID: cloneOptionalInt64(source.EnrollmentRequestChildID),
			SelectedWeekdays:         append([]int(nil), source.SelectedWeekdays...),
			Weekday:                  weekdayScopePtr(source.Weekday),
		}
		row.SetTenantID(tenantID)
		if err := s.deps.EnrollmentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: carry protected enrollment", Err: err}
		}
	}
	return nil
}

func protectedEnrollmentPeriodAfterRebase(sourcePeriodID, targetPeriodID *int64) *int64 {
	// Unscoped protected rows already apply to every selected period. Scoped
	// rows must follow an explicit successor period so materialization does not
	// silently filter the child out after an A→B edit.
	if sourcePeriodID != nil && targetPeriodID != nil {
		return cloneOptionalInt64(targetPeriodID)
	}
	return cloneOptionalInt64(sourcePeriodID)
}

// createStaffRoster writes the successor's supervisors. Explicit StaffIDs win
// (primary derived from PrimaryStaffID); nil carries over the previously-
// active roster with each row's is_primary flag preserved.
//
// Like createStudentRoster, the carry path dedupes per staff member: several
// active rows per staff (one per calendar period, migration 1.15.52) collapse
// to one successor row, preferring the row matching in.CalendarPeriodID. The
// preferred row's is_primary flag is kept.
func (s *TemplateSplitService) createStaffRoster(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	carried []*activitiesModel.SupervisorPlanned,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	rows := buildSuccessorStaffRows(in, carried, segmentValidUntil)
	for _, row := range rows {
		row.GroupID = groupID
		if row.ValidFrom.IsZero() || row.ValidFrom.Before(in.EffectiveDate) {
			row.ValidFrom = activitiesModel.Date(in.EffectiveDate)
		}
		row.ValidUntil = earliestActivityDate(row.ValidUntil, segmentValidUntil)
		row.CalendarPeriodID = in.CalendarPeriodID
		row.SetTenantID(tenantID)
		if err := s.deps.SupervisorRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: create supervisor", Err: err}
		}
	}
	return nil
}

func buildSuccessorStaffRows(
	in TemplateSplitInput,
	carried []*activitiesModel.SupervisorPlanned,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.SupervisorPlanned {
	if in.StaffIDs != nil {
		resolved, err := resolveTemplateRoster(in.Weekdays, nil, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
		if err != nil {
			// The handler validates the assignments before reaching the split,
			// so an error here means a caller bypassed that check; fall back to
			// the shared roster rather than dropping the staff entirely.
			resolved = resolvedTemplateRoster{Staff: sharedRosterRows(in.StaffIDs, in.PrimaryStaffID)}
		}
		rows := make([]*activitiesModel.SupervisorPlanned, 0, len(resolved.Staff))
		for _, row := range resolved.Staff {
			rows = append(rows, &activitiesModel.SupervisorPlanned{
				StaffID:   row.PersonID,
				IsPrimary: row.IsPrimary,
				Weekday:   weekdayScopePtr(row.Weekday),
			})
		}
		return rows
	}
	return carriedStaffRowsByPerson(carried, in.CalendarPeriodID, segmentValidUntil)
}

func carriedStaffRowsByPerson(
	carried []*activitiesModel.SupervisorPlanned,
	periodID *int64,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.SupervisorPlanned {
	// Keyed by (staff, weekday scope), not by staff alone: collapsing a
	// per-weekday roster (#2129) to one row per person would turn "Anna
	// montags, Bea dienstags" into "Anna and Bea every day" on the successor.
	byStaff := make(map[rosterPersonScope][]*activitiesModel.SupervisorPlanned, len(carried))
	order := make([]rosterPersonScope, 0, len(carried))
	for _, row := range carried {
		key := rosterPersonScope{PersonID: row.StaffID, Weekday: weekdayScopeKey(row.Weekday)}
		if _, seen := byStaff[key]; !seen {
			order = append(order, key)
		}
		byStaff[key] = append(byStaff[key], row)
	}
	rows := make([]*activitiesModel.SupervisorPlanned, 0, len(order))
	for _, key := range order {
		preferred := preferCarriedRow(byStaff[key], periodID, func(row *activitiesModel.SupervisorPlanned) *int64 { return row.CalendarPeriodID })
		rows = append(rows, &activitiesModel.SupervisorPlanned{
			StaffID:    key.PersonID,
			IsPrimary:  preferred.IsPrimary,
			ValidFrom:  preferred.ValidFrom,
			ValidUntil: earliestActivityDate(preferred.ValidUntil, segmentValidUntil),
			Weekday:    weekdayScopePtr(preferred.Weekday),
		})
	}
	return rows
}

// rosterPersonScope keys a carried roster row by person AND weekday scope.
type rosterPersonScope struct {
	PersonID int64
	// Weekday is 0 for the series-wide scope (a real ISO weekday is 1..7),
	// which makes the struct usable as a map key without a pointer.
	Weekday int
}

func weekdayScopeKey(weekday *int) int {
	if weekday == nil {
		return 0
	}
	return *weekday
}

// preferCarriedRow picks the carried roster row whose calendar_period_id
// matches the successor's target period (both nil counts as a match); when
// none matches it falls back to the first row. Generic over the two roster
// row types so the per-person selection rule lives in one place.
func preferCarriedRow[T any](rows []T, targetPeriodID *int64, periodOf func(T) *int64) T {
	for _, row := range rows {
		p := periodOf(row)
		if (p == nil && targetPeriodID == nil) ||
			(p != nil && targetPeriodID != nil && *p == *targetPeriodID) {
			return row
		}
	}
	return rows[0]
}

func earliestOptionalDate(left, right *timezone.Date) *timezone.Date {
	if left == nil {
		return cloneOptionalDate(right)
	}
	if right == nil || left.Before(*right) {
		return cloneOptionalDate(left)
	}
	return cloneOptionalDate(right)
}

func earliestActivityDate(left *activitiesModel.Date, right *timezone.Date) *activitiesModel.Date {
	return activityDatePtr(earliestOptionalDate(timezoneDatePtr(left), right))
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// unionSelectedWeekdays merges selected_weekdays across one student's carried
// rows. An empty/nil list means "all weekdays" and dominates the union; the
// result preserves ascending weekday order. With a single row this returns
// the preferred row's list unchanged.
func unionSelectedWeekdays(rows []*activitiesModel.StudentEnrollment, preferred *activitiesModel.StudentEnrollment) []int {
	if len(rows) == 1 {
		return preferred.SelectedWeekdays
	}
	seen := make(map[int]struct{})
	for _, e := range rows {
		if len(e.SelectedWeekdays) == 0 {
			return nil // "all weekdays" wins the union
		}
		for _, wd := range e.SelectedWeekdays {
			seen[wd] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for wd := activitiesModel.WeekdayMonday; wd <= activitiesModel.WeekdaySunday; wd++ {
		if _, ok := seen[wd]; ok {
			out = append(out, wd)
		}
	}
	return out
}
