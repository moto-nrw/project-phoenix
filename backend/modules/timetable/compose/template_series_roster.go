package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
)

// Roster reconciliation across split-series predecessors (#2187,
// "Nachzügler").
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

// reconcileSeriesPredecessorRoster mirrors the update's saved roster onto the
// bounded predecessor segments of the template's split series, from
// in.SeriesRosterFrom on. No-op without the anchor. Runs inside the update's
// tenant transaction, after the recurrence lock — the chain cannot shift
// between the segment read and the writes.
func (s *TemplateService) reconcileSeriesPredecessorRoster(
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
	if today := s.deps.Today(); anchor.Before(today) {
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

// weekdayScope carries the weekdays a reconciliation pass may act on: the ones
// this edit describes (edited) and every weekday the segment shares with the
// submitted recurrence (segment). They differ only for a per-weekday series
// (#2129), whose occurrence editor shows exactly one weekday's roster.
type weekdayScope struct {
	edited  []int
	segment []int
}

// reachesBeyondEdit reports whether a roster row also covers weekdays this
// edit does not describe. Such a row is left alone entirely: retiring it to
// satisfy the edited weekday would silently change the others too, while the
// create loop only writes rows for the edited weekdays.
func (w weekdayScope) reachesBeyondEdit(rowWeekday *int, segmentWeekdays, covered []int) bool {
	if len(w.edited) == len(w.segment) {
		return false
	}
	return len(coveredSeriesWeekdays(rowWeekday, segmentWeekdays, w.segment)) != len(covered)
}

// seriesWeekdayPerson keys a desired or satisfied roster membership.
type seriesWeekdayPerson struct {
	weekday  int
	personID int64
}

// predecessorWindow is the part of one predecessor segment the edit
// reconciles: [from, until) on the given weekdays.
type predecessorWindow struct {
	from     timezone.Date
	until    *timezone.Date
	weekdays weekdayScope
}

// predecessorWindowOf clips the segment to [max(anchor, segment valid_from),
// segment valid_until). The weekday universe is the PREDECESSOR's own
// recurrence: the edit's weekdays describe the living successor, which a split
// may have moved to other days entirely (#2187 review). A per-weekday series
// (#2129) is edited one weekday at a time, so the submitted roster describes
// only that weekday; judging the predecessor's other weekdays against it would
// retire rows nobody touched.
func predecessorWindowOf(seg *templateSeriesSegment, anchor timezone.Date, scopeWeekdays []int) (predecessorWindow, bool) {
	rowFrom := anchor
	if seg.ValidFrom != nil && rowFrom.Before(*seg.ValidFrom) {
		rowFrom = *seg.ValidFrom
	}
	if !rowFrom.Before(*seg.ValidUntil) {
		return predecessorWindow{}, false
	}
	segmentWeekdays := uniqueSortedWeekdays(seg.Weekdays)
	if len(segmentWeekdays) == 0 {
		return predecessorWindow{}, false
	}
	scoped := segmentWeekdays
	if len(scopeWeekdays) > 0 {
		scoped = intersectWeekdays(segmentWeekdays, scopeWeekdays)
		if len(scoped) == 0 {
			return predecessorWindow{}, false
		}
	}
	return predecessorWindow{
		from:     rowFrom,
		until:    seg.ValidUntil,
		weekdays: weekdayScope{edited: scoped, segment: segmentWeekdays},
	}, true
}

// rowCoverage returns the edited weekdays an existing roster row of the
// predecessor covers, or false when the diff leaves the row alone: a person
// outside the edit's scope, another period's row, a row reaching beyond the
// edited weekdays, or one outside the window.
func (w predecessorWindow) rowCoverage(
	row predecessorRosterRow,
	scope map[int64]struct{},
	targetPeriodID *int64,
	segmentWeekdays []int,
) ([]int, bool) {
	if _, inScope := scope[row.personID]; !inScope {
		return nil, false
	}
	if !rosterPeriodApplies(row.periodID, targetPeriodID) {
		return nil, false
	}
	covered := coveredSeriesWeekdays(row.weekday, segmentWeekdays, w.weekdays.edited)
	if len(covered) == 0 || w.weekdays.reachesBeyondEdit(row.weekday, segmentWeekdays, covered) {
		return nil, false
	}
	if !validityWindowsOverlap(row.validFrom, row.validUntil, w.from, w.until) {
		return nil, false
	}
	return covered, true
}

// staysCorrect reports whether the row already covers the window exactly
// with the wanted membership on every covered weekday.
func (w predecessorWindow) staysCorrect(row predecessorRosterRow, covered []int, want map[seriesWeekdayPerson]bool) bool {
	return seriesRowStaysCorrect(row.validFrom, row.validUntil, w.from, w.until) &&
		wantedOnAllWeekdays(want, covered, row.personID, false)
}

// predecessorRosterRow is the part of an enrollment or supervisor row the
// predecessor diff judges.
type predecessorRosterRow struct {
	personID   int64
	periodID   *int64
	weekday    *int
	validFrom  timezone.Date
	validUntil *timezone.Date
}

func enrollmentRosterRow(row *activitiesModel.StudentEnrollment) predecessorRosterRow {
	return predecessorRosterRow{
		personID: row.StudentID, periodID: row.CalendarPeriodID, weekday: row.Weekday,
		validFrom: timezone.Date(row.ValidFrom), validUntil: timezoneDatePtr(row.ValidUntil),
	}
}

func supervisorRosterRow(row *activitiesModel.SupervisorPlanned) predecessorRosterRow {
	return predecessorRosterRow{
		personID: row.StaffID, periodID: row.CalendarPeriodID, weekday: row.Weekday,
		validFrom: timezone.Date(row.ValidFrom), validUntil: timezoneDatePtr(row.ValidUntil),
	}
}

// reconcilePredecessorSegmentRoster aligns ONE bounded predecessor segment's
// enrollment and supervisor rows (and their already-materialized occurrences)
// with the desired roster, inside [max(anchor, segment valid_from), segment
// valid_until) — restricted to the people the edit actually changed.
func (s *TemplateService) reconcilePredecessorSegmentRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	seg *templateSeriesSegment,
	anchor timezone.Date,
	roster resolvedTemplateRoster,
	studentScope, staffScope map[int64]struct{},
) error {
	window, ok := predecessorWindowOf(seg, anchor, in.SeriesRosterScopeWeekdays)
	if !ok {
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
		protectedPredecessorEnrollments(priorEnrollments, window.from, window.until),
		window.weekdays.edited,
		func(row *activitiesModel.StudentEnrollment) bool {
			return rosterPeriodApplies(row.CalendarPeriodID, in.CalendarPeriodID)
		},
	)
	wantStudents := desiredSeriesRoster(
		excludeProtectedStudentWeekdays(roster.Students, protectedCoverage), window.weekdays.edited)
	wantStaff := desiredSeriesRoster(roster.Staff, window.weekdays.edited)

	touchedStudents, err := s.reconcilePredecessorEnrollmentRows(ctx, in, tenantID, seg, window, wantStudents, priorEnrollments, studentScope)
	if err != nil {
		return err
	}
	touchedStaff, err := s.reconcilePredecessorSupervisorRows(ctx, in, tenantID, seg, window, roster.Staff, wantStaff, priorSupervisors, staffScope)
	if err != nil {
		return err
	}
	return s.reconcilePredecessorOccurrences(ctx, seg.Group.ID, window.from, touchedStudents, priorEnrollments, touchedStaff, priorSupervisors)
}

// reconcilePredecessorOccurrences carries the roster writes onto the
// segment's already-materialized occurrences.
func (s *TemplateService) reconcilePredecessorOccurrences(
	ctx context.Context,
	templateID int64,
	from timezone.Date,
	touchedStudents []int64,
	priorEnrollments []*activitiesModel.StudentEnrollment,
	touchedStaff []int64,
	priorSupervisors []*activitiesModel.SupervisorPlanned,
) error {
	if len(touchedStudents) > 0 && s.deps.ActivityInstanceRepo != nil && s.deps.InstanceStudentRepo != nil {
		if _, _, err := s.newRosterReconciler().reconcileSourcedTemplateRosters(
			ctx, templateID, touchedStudents, from, priorEnrollments,
		); err != nil {
			return &ScheduleError{Op: "update template: reconcile predecessor occurrences", Err: err}
		}
	}
	if len(touchedStaff) > 0 && s.deps.ActivityInstanceRepo != nil && s.deps.InstanceStaffRepo != nil {
		if err := s.reconcilePredecessorInstanceStaff(ctx, templateID, touchedStaff, from, priorSupervisors); err != nil {
			return err
		}
	}
	return nil
}

// reconcilePredecessorEnrollmentRows diffs the segment's enrollment rows
// against the desired student roster and returns the students whose coverage
// changed (rows created, closed, or deleted). Students outside scope — the
// people this edit actually changed — are ignored entirely.
func (s *TemplateService) reconcilePredecessorEnrollmentRows(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	seg *templateSeriesSegment,
	window predecessorWindow,
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
		judged := enrollmentRosterRow(row)
		covered, ok := window.rowCoverage(judged, scope, in.CalendarPeriodID, seg.Weekdays)
		if !ok {
			continue
		}
		if window.staysCorrect(judged, covered, want) {
			markSatisfied(satisfied, covered, row.StudentID)
			continue
		}
		retired, err := s.retirePredecessorEnrollment(ctx, row, judged, window)
		if err != nil {
			return nil, err
		}
		if retired {
			touched[row.StudentID] = true
		}
	}

	for _, key := range missingSeriesWants(want, satisfied, scope) {
		weekday := key.weekday
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        key.personID,
			ActivityGroupID:  seg.Group.ID,
			ValidFrom:        activitiesModel.Date(window.from),
			ValidUntil:       activityDatePtr(window.until),
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

// retirePredecessorEnrollment closes a row at the window start or deletes it
// when it only started there; an unrelated bounded window survives untouched
// and reports false.
func (s *TemplateService) retirePredecessorEnrollment(
	ctx context.Context,
	row *activitiesModel.StudentEnrollment,
	judged predecessorRosterRow,
	window predecessorWindow,
) (bool, error) {
	switch classifyOwnedRosterRetirement(judged.validFrom, judged.validUntil, true, window.from, window.until) {
	case rosterRetirementDelete:
		if err := s.deps.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return false, &ScheduleError{Op: "update template: delete predecessor enrollment", Err: err}
		}
	case rosterRetirementClose:
		if err := s.deps.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, activitiesModel.Date(window.from)); err != nil {
			return false, &ScheduleError{Op: "update template: close predecessor enrollment", Err: err}
		}
	default:
		return false, nil
	}
	return true, nil
}

// reconcilePredecessorSupervisorRows is the staff twin of the enrollment diff.
// A kept row must also match the desired is_primary flag on every weekday it
// covers, so a primary change propagates like a membership change.
func (s *TemplateService) reconcilePredecessorSupervisorRows(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	seg *templateSeriesSegment,
	window predecessorWindow,
	staffRows []resolvedRosterRow,
	want map[seriesWeekdayPerson]bool,
	priorRows []*activitiesModel.SupervisorPlanned,
	scope map[int64]struct{},
) ([]int64, error) {
	primaryWanted := predecessorPrimaryWanted(in.SeriesRosterPrimaryChanged, staffRows, window.weekdays.edited)
	satisfied := make(map[seriesWeekdayPerson]bool)
	touched := make(map[int64]bool)
	for _, row := range priorRows {
		if row == nil {
			continue
		}
		judged := supervisorRosterRow(row)
		covered, ok := window.rowCoverage(judged, scope, in.CalendarPeriodID, seg.Weekdays)
		if !ok {
			continue
		}
		if window.staysCorrect(judged, covered, want) &&
			supervisorPrimaryStaysCorrect(in.SeriesRosterPrimaryChanged, primaryWanted, covered, row) {
			markSatisfied(satisfied, covered, row.StaffID)
			continue
		}
		retired, err := s.retirePredecessorSupervisor(ctx, row, judged, window)
		if err != nil {
			return nil, err
		}
		if retired {
			touched[row.StaffID] = true
		}
	}

	for _, key := range missingSeriesWants(want, satisfied, scope) {
		weekday := key.weekday
		supervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          key.personID,
			GroupID:          seg.Group.ID,
			IsPrimary:        primaryWanted[key],
			ValidFrom:        activitiesModel.Date(window.from),
			ValidUntil:       activityDatePtr(window.until),
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

// predecessorPrimaryWanted collects the (weekday, staff) pairs that should
// carry the Hauptbetreuung. The living segment's lead must not be stamped onto
// a predecessor row: the weekday- and period-scoped row would outrank the
// existing shared one, and the ensure_single_primary_supervisor trigger clears
// the old flag outright. The flag therefore only travels when the edit
// actually changed the Hauptbetreuung (#2187 review).
func predecessorPrimaryWanted(primaryChanged bool, staffRows []resolvedRosterRow, scoped []int) map[seriesWeekdayPerson]bool {
	primaryWanted := make(map[seriesWeekdayPerson]bool)
	if !primaryChanged {
		return primaryWanted
	}
	for _, row := range staffRows {
		if !row.IsPrimary {
			continue
		}
		for _, weekday := range coveredSeriesWeekdays(row.Weekday, scoped, scoped) {
			primaryWanted[seriesWeekdayPerson{weekday: weekday, personID: row.PersonID}] = true
		}
	}
	return primaryWanted
}

// retirePredecessorSupervisor is the staff twin of retirePredecessorEnrollment.
func (s *TemplateService) retirePredecessorSupervisor(
	ctx context.Context,
	row *activitiesModel.SupervisorPlanned,
	judged predecessorRosterRow,
	window predecessorWindow,
) (bool, error) {
	switch classifyOwnedRosterRetirement(judged.validFrom, judged.validUntil, true, window.from, window.until) {
	case rosterRetirementDelete:
		if err := s.deps.ActivitySupervisorRepo.Delete(ctx, row.ID); err != nil {
			return false, &ScheduleError{Op: "update template: delete predecessor supervisor", Err: err}
		}
	case rosterRetirementClose:
		if err := s.deps.ActivitySupervisorRepo.SetValidUntilByID(ctx, row.ID, activitiesModel.Date(window.from)); err != nil {
			return false, &ScheduleError{Op: "update template: close predecessor supervisor", Err: err}
		}
	default:
		return false, nil
	}
	return true, nil
}

// supervisorPrimaryStaysCorrect reports whether a kept supervisor row still
// carries the wanted Hauptbetreuung flag; without a primary change the flag
// is not judged.
func supervisorPrimaryStaysCorrect(
	primaryChanged bool,
	primaryWanted map[seriesWeekdayPerson]bool,
	covered []int,
	row *activitiesModel.SupervisorPlanned,
) bool {
	return !primaryChanged || primaryFlagMatches(primaryWanted, covered, row.StaffID, row.IsPrimary)
}

// missingSeriesWants returns the wanted (weekday, person) pairs no kept row
// satisfies, restricted to the people the edit changed, in stable order.
func missingSeriesWants(
	want, satisfied map[seriesWeekdayPerson]bool,
	scope map[int64]struct{},
) []seriesWeekdayPerson {
	missing := make([]seriesWeekdayPerson, 0, len(want))
	for _, key := range sortedSeriesWants(want) {
		if satisfied[key] {
			continue
		}
		if _, inScope := scope[key.personID]; inScope {
			missing = append(missing, key)
		}
	}
	return missing
}

func markSatisfied(satisfied map[seriesWeekdayPerson]bool, weekdays []int, personID int64) {
	for _, weekday := range weekdays {
		satisfied[seriesWeekdayPerson{weekday: weekday, personID: personID}] = true
	}
}
