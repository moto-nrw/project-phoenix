// Package schedule — roster reconciliation across split-series predecessors
// (#2187, "Nachzügler").
//
// A roster change made "ab Datum D" must be true on every occurrence from D
// on, including occurrences that belong to an already-capped predecessor
// segment of the same split series: a child enrolled from Tue 18.08. has to
// stand on the 18.08. list even when the first school week still materialized
// from the capped predecessor. PUT /timetable/templates/{id} therefore accepts
// an optional series_roster_from anchor plus the scope of people the edit
// actually changed; when both are set, that change is mirrored onto the
// bounded predecessor segments overlapping [anchor, living valid_from):
//
//   - added people gain a bounded, weekday-explicit row
//     [max(anchor, segment valid_from), segment valid_until)
//   - removed people have their segment rows closed at the anchor (or deleted
//     when they only started there)
//   - already-correct rows, protected rows (enrollment_request_child_id,
//     selected_weekdays), other-period rows, and unrelated bounded windows are
//     never touched
//
// Everyone outside the submitted scope is left alone on purpose: the roster
// written to the living segment describes THAT segment (a split may have
// changed it), so it is not the predecessor's target set. Reconciling against
// it wholesale would retire predecessor-only members nobody touched.
//
// The anchor does NOT change how the living segment itself is written —
// templateRosterValidFrom stays authoritative there; this pass only extends
// the change backwards over the chain. Non-roster fields (name, time, room,
// weekdays) deliberately do not retro-apply to predecessor windows: those
// segments' recurrence is closed history, only roster membership is a fact
// about children and staff that must be right on the day.
//
// Because the materializer is insert-only, the enrollment/supervisor writes
// alone would not change already-planned occurrences. The same surgical
// instance reconcilers the offering resync uses close that gap (students via
// RosterReconciler.ReconcileSourcedTemplateRosters, staff via the mirror pass
// below); a wholesale re-plan of the predecessor window would wipe
// per-occurrence hand edits and is deliberately avoided.
package schedule

import (
	"context"
	"log/slog"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// reconcileSeriesPredecessorRoster mirrors the update's saved roster onto the
// bounded predecessor segments of the template's split series, from
// in.SeriesRosterFrom on. No-op without the anchor. Runs inside the update's
// tenant transaction, after lockTenantRecurrenceWrites — the chain cannot
// shift between the segment read and the writes.
func (s *TimetableDataService) reconcileSeriesPredecessorRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	livingValidFrom *timezone.Date,
) error {
	if in.SeriesRosterFrom == nil {
		return nil
	}
	studentScope := int64Set(in.SeriesRosterScopeStudentIDs)
	staffScope := int64Set(in.SeriesRosterScopeStaffIDs)
	if len(studentScope) == 0 && len(staffScope) == 0 {
		// The anchor alone says nothing about WHO changed, and the submitted
		// roster is not the predecessor's target set — without a scope there is
		// nothing that may safely be mirrored.
		return nil
	}
	anchor := *in.SeriesRosterFrom
	if today := timezone.TodayDate(); anchor.Before(today) {
		// History is never rewritten; a stale client anchor degrades to "from
		// today on" instead of erroring a save that is otherwise valid.
		anchor = today
	}
	segments, err := loadTemplateSeriesSegments(
		ctx,
		s.deps.ActivityGroupRepo,
		s.deps.ActivityScheduleRepo,
		s.getLogger(),
		in.TemplateID,
	)
	if err != nil {
		return err
	}
	roster, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
	if err != nil {
		return &ScheduleError{Op: "update template: resolve series roster", Err: err}
	}

	reconciled := 0
	for i := range segments {
		seg := &segments[i]
		if !segmentIsReconcilablePredecessor(seg, in.TemplateID, anchor, livingValidFrom) {
			continue
		}
		if err := s.reconcilePredecessorSegmentRoster(ctx, in, tenantID, seg, anchor, roster, studentScope, staffScope); err != nil {
			return err
		}
		reconciled++
	}
	if reconciled > 0 {
		s.getLogger().Info("reconciled split-series predecessor rosters",
			slog.Int64("template_id", in.TemplateID),
			slog.String("series_roster_from", anchor.String()),
			slog.Int("segments", reconciled),
		)
	}
	return nil
}

// segmentIsReconcilablePredecessor filters the lineage down to the bounded
// predecessor segments whose window still reaches into [anchor, ∞) and lies
// before the living segment. Empty windows (valid_from == valid_until — the
// degenerate shape a same-day split leaves behind) materialize nothing and
// are skipped.
func segmentIsReconcilablePredecessor(
	seg *templateSeriesSegment,
	livingTemplateID int64,
	anchor timezone.Date,
	livingValidFrom *timezone.Date,
) bool {
	if seg.Group == nil || seg.Group.ID == livingTemplateID || seg.ValidUntil == nil {
		return false
	}
	if seg.ValidFrom != nil && !seg.ValidFrom.Before(*seg.ValidUntil) {
		return false // empty window
	}
	if !seg.ValidUntil.After(anchor) {
		return false // the segment ended before the change takes effect
	}
	if livingValidFrom != nil && seg.ValidFrom != nil && !seg.ValidFrom.Before(*livingValidFrom) {
		return false
	}
	return true
}

// seriesWeekdayPerson keys a desired or satisfied roster membership.
type seriesWeekdayPerson struct {
	weekday  int
	personID int64
}

// reconcilePredecessorSegmentRoster aligns ONE bounded predecessor segment's
// enrollment and supervisor rows (and their already-materialized occurrences)
// with the desired roster, inside [max(anchor, segment valid_from), segment
// valid_until) — restricted to the people the edit actually changed.
func (s *TimetableDataService) reconcilePredecessorSegmentRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	seg *templateSeriesSegment,
	anchor timezone.Date,
	roster resolvedTemplateRoster,
	studentScope, staffScope map[int64]struct{},
) error {
	rowFrom := anchor
	if seg.ValidFrom != nil && rowFrom.Before(*seg.ValidFrom) {
		rowFrom = *seg.ValidFrom
	}
	rowUntil := seg.ValidUntil
	if !rowFrom.Before(*rowUntil) {
		return nil
	}
	scoped := intersectWeekdays(seg.Weekdays, in.Weekdays)
	if len(scoped) == 0 {
		// The predecessor runs on none of the submitted weekdays — weekday
		// changes are a non-roster edit and never retro-apply.
		return nil
	}

	// Snapshots BEFORE any write: they classify the existing rows and double
	// as the prior state for the instance reconcilers, which is what keeps a
	// child staff had hand-removed from one occurrence from being resurrected.
	priorEnrollments, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, seg.Group.ID)
	if err != nil {
		return &ScheduleError{Op: "update template: load predecessor enrollments", Err: err}
	}
	priorSupervisors, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, seg.Group.ID)
	if err != nil {
		return &ScheduleError{Op: "update template: load predecessor supervisors", Err: err}
	}

	// A weekday already supplied by an externally-owned row (care offering,
	// weekday selection) must not gain a second, editor-owned row — the same
	// rule the canonical template write applies.
	protectedCoverage := buildProtectedStudentCoverage(
		protectedPredecessorEnrollments(priorEnrollments, rowFrom, rowUntil),
		scoped,
		func(row *activitiesModel.StudentEnrollment) bool {
			return rosterPeriodApplies(row.CalendarPeriodID, in.CalendarPeriodID)
		},
	)
	wantStudents := desiredSeriesRoster(
		excludeProtectedStudentWeekdays(roster.Students, protectedCoverage), scoped)
	wantStaff := desiredSeriesRoster(roster.Staff, scoped)

	touchedStudents, err := s.reconcilePredecessorEnrollmentRows(
		ctx, in, tenantID, seg, rowFrom, rowUntil, scoped, wantStudents, priorEnrollments, studentScope,
	)
	if err != nil {
		return err
	}
	touchedStaff, err := s.reconcilePredecessorSupervisorRows(
		ctx, in, tenantID, seg, rowFrom, rowUntil, scoped, roster.Staff, wantStaff, priorSupervisors, staffScope,
	)
	if err != nil {
		return err
	}

	if len(touchedStudents) > 0 && s.deps.ActivityInstanceRepo != nil && s.deps.InstanceStudentRepo != nil {
		reconciler := NewRosterReconciler(s.deps.ActivityInstanceRepo, s.deps.InstanceStudentRepo, s.deps.StudentEnrollmentRepo, s.deps.Logger)
		if _, _, err := reconciler.ReconcileSourcedTemplateRosters(
			ctx, seg.Group.ID, touchedStudents, rowFrom, priorEnrollments,
		); err != nil {
			return &ScheduleError{Op: "update template: reconcile predecessor occurrences", Err: err}
		}
	}
	if len(touchedStaff) > 0 && s.deps.ActivityInstanceRepo != nil && s.deps.InstanceStaffRepo != nil {
		if err := s.reconcilePredecessorInstanceStaff(ctx, seg.Group.ID, touchedStaff, rowFrom, priorSupervisors); err != nil {
			return err
		}
	}
	return nil
}

// reconcilePredecessorEnrollmentRows diffs the segment's enrollment rows
// against the desired student roster and returns the students whose coverage
// changed (rows created, closed, or deleted). Students outside scope — the
// people this edit actually changed — are ignored entirely.
func (s *TimetableDataService) reconcilePredecessorEnrollmentRows(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	seg *templateSeriesSegment,
	rowFrom timezone.Date,
	rowUntil *timezone.Date,
	scoped []int,
	want map[seriesWeekdayPerson]bool,
	priorRows []*activitiesModel.StudentEnrollment,
	scope map[int64]struct{},
) ([]int64, error) {
	satisfied := make(map[seriesWeekdayPerson]bool)
	touched := make(map[int64]bool)
	for _, row := range priorRows {
		if row == nil || enrollmentIsProtected(row) {
			continue
		}
		if _, inScope := scope[row.StudentID]; !inScope {
			continue
		}
		if !rosterPeriodApplies(row.CalendarPeriodID, in.CalendarPeriodID) {
			continue
		}
		covered := coveredSeriesWeekdays(row.Weekday, seg.Weekdays, scoped)
		if len(covered) == 0 {
			continue
		}
		if !validityWindowsOverlap(row.ValidFrom, row.ValidUntil, rowFrom, rowUntil) {
			continue
		}
		if seriesRowStaysCorrect(row.ValidFrom, row.ValidUntil, rowFrom, rowUntil) &&
			wantedOnAllWeekdays(want, covered, row.StudentID, false) {
			for _, weekday := range covered {
				satisfied[seriesWeekdayPerson{weekday: weekday, personID: row.StudentID}] = true
			}
			continue
		}
		action := classifyOwnedRosterRetirement(row.ValidFrom, row.ValidUntil, true, rowFrom, rowUntil)
		switch action {
		case rosterRetirementDelete:
			if err := s.deps.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
				return nil, &ScheduleError{Op: "update template: delete predecessor enrollment", Err: err}
			}
		case rosterRetirementClose:
			if err := s.deps.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, rowFrom); err != nil {
				return nil, &ScheduleError{Op: "update template: close predecessor enrollment", Err: err}
			}
		default:
			continue // unrelated bounded window — survives untouched
		}
		touched[row.StudentID] = true
	}

	for _, key := range sortedSeriesWants(want) {
		if satisfied[key] {
			continue
		}
		if _, inScope := scope[key.personID]; !inScope {
			continue
		}
		weekday := key.weekday
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        key.personID,
			ActivityGroupID:  seg.Group.ID,
			ValidFrom:        rowFrom,
			ValidUntil:       cloneOptionalDate(rowUntil),
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          &weekday,
		}
		enrollment.SetTenantID(tenantID)
		if err := s.deps.StudentEnrollmentRepo.Create(ctx, enrollment); err != nil {
			return nil, &ScheduleError{Op: "update template: create predecessor enrollment", Err: err}
		}
		touched[key.personID] = true
	}
	return sortedInt64Keys(touched), nil
}

// reconcilePredecessorSupervisorRows is the staff twin of the enrollment diff.
// A kept row must also match the desired is_primary flag on every weekday it
// covers, so a primary change propagates like a membership change.
func (s *TimetableDataService) reconcilePredecessorSupervisorRows(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	seg *templateSeriesSegment,
	rowFrom timezone.Date,
	rowUntil *timezone.Date,
	scoped []int,
	staffRows []resolvedRosterRow,
	want map[seriesWeekdayPerson]bool,
	priorRows []*activitiesModel.SupervisorPlanned,
	scope map[int64]struct{},
) ([]int64, error) {
	primaryWanted := make(map[seriesWeekdayPerson]bool)
	for _, row := range staffRows {
		for _, weekday := range coveredSeriesWeekdays(row.Weekday, scoped, scoped) {
			if row.IsPrimary {
				primaryWanted[seriesWeekdayPerson{weekday: weekday, personID: row.PersonID}] = true
			}
		}
	}

	satisfied := make(map[seriesWeekdayPerson]bool)
	touched := make(map[int64]bool)
	for _, row := range priorRows {
		if row == nil {
			continue
		}
		if _, inScope := scope[row.StaffID]; !inScope {
			continue
		}
		if !rosterPeriodApplies(row.CalendarPeriodID, in.CalendarPeriodID) {
			continue
		}
		covered := coveredSeriesWeekdays(row.Weekday, seg.Weekdays, scoped)
		if len(covered) == 0 {
			continue
		}
		if !validityWindowsOverlap(row.ValidFrom, row.ValidUntil, rowFrom, rowUntil) {
			continue
		}
		if seriesRowStaysCorrect(row.ValidFrom, row.ValidUntil, rowFrom, rowUntil) &&
			wantedOnAllWeekdays(want, covered, row.StaffID, false) &&
			primaryFlagMatches(primaryWanted, covered, row.StaffID, row.IsPrimary) {
			for _, weekday := range covered {
				satisfied[seriesWeekdayPerson{weekday: weekday, personID: row.StaffID}] = true
			}
			continue
		}
		action := classifyOwnedRosterRetirement(row.ValidFrom, row.ValidUntil, true, rowFrom, rowUntil)
		switch action {
		case rosterRetirementDelete:
			if err := s.deps.ActivitySupervisorRepo.Delete(ctx, row.ID); err != nil {
				return nil, &ScheduleError{Op: "update template: delete predecessor supervisor", Err: err}
			}
		case rosterRetirementClose:
			if err := s.deps.ActivitySupervisorRepo.SetValidUntilByID(ctx, row.ID, rowFrom); err != nil {
				return nil, &ScheduleError{Op: "update template: close predecessor supervisor", Err: err}
			}
		default:
			continue
		}
		touched[row.StaffID] = true
	}

	for _, key := range sortedSeriesWants(want) {
		if satisfied[key] {
			continue
		}
		if _, inScope := scope[key.personID]; !inScope {
			continue
		}
		weekday := key.weekday
		supervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          key.personID,
			GroupID:          seg.Group.ID,
			IsPrimary:        primaryWanted[key],
			ValidFrom:        rowFrom,
			ValidUntil:       cloneOptionalDate(rowUntil),
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          &weekday,
		}
		supervisor.SetTenantID(tenantID)
		if err := s.deps.ActivitySupervisorRepo.Create(ctx, supervisor); err != nil {
			return nil, &ScheduleError{Op: "update template: create predecessor supervisor", Err: err}
		}
		touched[key.personID] = true
	}
	return sortedInt64Keys(touched), nil
}

// reconcilePredecessorInstanceStaff aligns schedule.instance_staff on ONE
// template's already-materialized, still-planned occurrences with the
// template's current activities.supervisors rows, restricted to the given
// staff — the supervisor twin of ReconcileSourcedTemplateRosters. Rows
// recording a real decision (a substitution, an absence, a sick-cascade
// stamp) are never touched, and a staff member a planner had removed from one
// occurrence by hand is not resurrected (priorSupervisors semantics).
func (s *TimetableDataService) reconcilePredecessorInstanceStaff(
	ctx context.Context,
	templateID int64,
	staffIDs []int64,
	from timezone.Date,
	priorSupervisors []*activitiesModel.SupervisorPlanned,
) error {
	if templateID <= 0 || len(staffIDs) == 0 {
		return nil
	}
	if today := timezone.TodayDate(); from.Before(today) {
		from = today
	}
	all, err := s.deps.ActivityInstanceRepo.FindPlannedTemplateBackedFrom(ctx, from)
	if err != nil {
		return &ScheduleError{Op: "reconcile predecessor staff: load future instances", Err: err}
	}
	instances := make([]*scheduleModel.ActivityInstance, 0, len(all))
	instanceIDs := make([]int64, 0, len(all))
	for _, inst := range all {
		if inst.ActivityGroupID == nil || *inst.ActivityGroupID != templateID {
			continue
		}
		instances = append(instances, inst)
		instanceIDs = append(instanceIDs, inst.ID)
	}
	if len(instances) == 0 {
		return nil
	}

	supervisors, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return &ScheduleError{Op: "reconcile predecessor staff: load supervisors", Err: err}
	}
	byStaff := make(map[int64][]*activitiesModel.SupervisorPlanned, len(supervisors))
	for _, sup := range supervisors {
		byStaff[sup.StaffID] = append(byStaff[sup.StaffID], sup)
	}
	priorByStaff := make(map[int64][]*activitiesModel.SupervisorPlanned, len(priorSupervisors))
	for _, sup := range priorSupervisors {
		priorByStaff[sup.StaffID] = append(priorByStaff[sup.StaffID], sup)
	}

	existingRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return &ScheduleError{Op: "reconcile predecessor staff: load existing rows", Err: err}
	}
	existing := make(map[instanceStudentPair]*scheduleModel.InstanceStaff, len(existingRows))
	for _, row := range existingRows {
		existing[instanceStudentPair{instanceID: row.InstanceID, studentID: row.StaffID}] = row
	}

	created, removed := 0, 0
	for _, inst := range instances {
		periodID := int64(0)
		if inst.CalendarPeriodID != nil {
			periodID = *inst.CalendarPeriodID
		}
		primaryStaffID, hasPrimary := effectivePrimarySupervisor(supervisors, inst.Date, periodID)
		for _, staffID := range staffIDs {
			desired := supervisorsPlanStaffOn(byStaff[staffID], inst.Date, periodID)
			key := instanceStudentPair{instanceID: inst.ID, studentID: staffID}
			row, exists := existing[key]
			switch {
			case desired && !exists:
				if supervisorsPlanStaffOn(priorByStaff[staffID], inst.Date, periodID) {
					// The pre-edit rows already planned this occurrence and the
					// row is gone regardless — a hand removal that stays.
					continue
				}
				fresh := &scheduleModel.InstanceStaff{
					InstanceID: inst.ID,
					StaffID:    staffID,
					IsPrimary:  hasPrimary && staffID == primaryStaffID,
				}
				if err := s.deps.InstanceStaffRepo.Create(ctx, fresh); err != nil {
					return &ScheduleError{Op: "reconcile predecessor staff: add staff", Err: err}
				}
				existing[key] = fresh
				created++
			case !desired && exists && instanceStaffRowIsStillPlanned(row):
				if err := s.deps.InstanceStaffRepo.Delete(ctx, row.ID); err != nil {
					return &ScheduleError{Op: "reconcile predecessor staff: remove staff", Err: err}
				}
				delete(existing, key)
				removed++
			}
		}
	}
	if created > 0 || removed > 0 {
		s.getLogger().Info("reconciled materialized staff after series roster change",
			slog.Int64("template_id", templateID),
			slog.Int("staff", len(staffIDs)),
			slog.Int("rows_created", created),
			slog.Int("rows_removed", removed),
		)
	}
	return nil
}

// supervisorsPlanStaffOn reports whether any of the rows plans its staff
// member on the occurrence date.
func supervisorsPlanStaffOn(rows []*activitiesModel.SupervisorPlanned, date timezone.Date, periodID int64) bool {
	for _, sup := range rows {
		if isSupervisorValidOn(sup, date, periodID) {
			return true
		}
	}
	return false
}

// instanceStaffRowIsStillPlanned reports whether a staff assignment is still
// purely a plan: no substitution, no absence, no sick-cascade stamp. Only such
// rows may be removed when the roster stops covering the staff member.
func instanceStaffRowIsStillPlanned(row *scheduleModel.InstanceStaff) bool {
	return !row.IsSubstitute && !row.IsAbsent && row.AbsenceReason == nil && row.SickAbsenceID == nil
}

// desiredSeriesRoster expands the resolved roster rows into (weekday, person)
// wants inside the scoped weekdays. A nil-weekday row applies to every scoped
// weekday; an explicit row only to its own.
func desiredSeriesRoster(rows []resolvedRosterRow, scoped []int) map[seriesWeekdayPerson]bool {
	want := make(map[seriesWeekdayPerson]bool)
	for _, row := range rows {
		for _, weekday := range coveredSeriesWeekdays(row.Weekday, scoped, scoped) {
			want[seriesWeekdayPerson{weekday: weekday, personID: row.PersonID}] = true
		}
	}
	return want
}

// coveredSeriesWeekdays returns the scoped weekdays a roster row covers: all
// of them for a shared (nil weekday) row bounded by the row's own universe,
// or the single explicit weekday when it is scoped.
func coveredSeriesWeekdays(rowWeekday *int, universe, scoped []int) []int {
	if rowWeekday != nil {
		if slices.Contains(scoped, *rowWeekday) {
			return []int{*rowWeekday}
		}
		return nil
	}
	return intersectWeekdays(universe, scoped)
}

func intersectWeekdays(left, right []int) []int {
	rightSet := make(map[int]struct{}, len(right))
	for _, weekday := range right {
		rightSet[weekday] = struct{}{}
	}
	out := make([]int, 0, len(left))
	for _, weekday := range uniqueSortedWeekdays(left) {
		if _, ok := rightSet[weekday]; ok {
			out = append(out, weekday)
		}
	}
	return out
}

// seriesRowStaysCorrect reports whether an existing row already covers the
// reconciliation window exactly: it starts at or before the window and ends
// with the segment.
func seriesRowStaysCorrect(validFrom timezone.Date, validUntil *timezone.Date, rowFrom timezone.Date, rowUntil *timezone.Date) bool {
	return !validFrom.After(rowFrom) && optionalDatesEqual(validUntil, rowUntil)
}

func wantedOnAllWeekdays(want map[seriesWeekdayPerson]bool, weekdays []int, personID int64, whenEmpty bool) bool {
	if len(weekdays) == 0 {
		return whenEmpty
	}
	for _, weekday := range weekdays {
		if !want[seriesWeekdayPerson{weekday: weekday, personID: personID}] {
			return false
		}
	}
	return true
}

func primaryFlagMatches(primaryWanted map[seriesWeekdayPerson]bool, weekdays []int, personID int64, isPrimary bool) bool {
	for _, weekday := range weekdays {
		if primaryWanted[seriesWeekdayPerson{weekday: weekday, personID: personID}] != isPrimary {
			return false
		}
	}
	return true
}

func sortedSeriesWants(want map[seriesWeekdayPerson]bool) []seriesWeekdayPerson {
	keys := make([]seriesWeekdayPerson, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].weekday != keys[j].weekday {
			return keys[i].weekday < keys[j].weekday
		}
		return keys[i].personID < keys[j].personID
	})
	return keys
}

// int64Set indexes the submitted reconciliation scope; non-positive ids are
// dropped so a malformed payload cannot widen it.
func int64Set(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	return set
}

// protectedPredecessorEnrollments returns the externally-owned rows of a
// segment that overlap the reconciliation window — the rows whose weekday
// coverage the editor may not duplicate.
func protectedPredecessorEnrollments(
	rows []*activitiesModel.StudentEnrollment,
	rowFrom timezone.Date,
	rowUntil *timezone.Date,
) []*activitiesModel.StudentEnrollment {
	out := make([]*activitiesModel.StudentEnrollment, 0, len(rows))
	for _, row := range rows {
		if row == nil || !enrollmentIsProtected(row) {
			continue
		}
		if !validityWindowsOverlap(row.ValidFrom, row.ValidUntil, rowFrom, rowUntil) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func sortedInt64Keys(set map[int64]bool) []int64 {
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
