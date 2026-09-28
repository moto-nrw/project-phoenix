package compose

import (
	"context"
	"log/slog"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// instanceStaffReconciliation is the state one staff alignment pass works
// against: the template's current and prior supervisor rows and the
// occurrences' existing staff rows.
type instanceStaffReconciliation struct {
	supervisors  []*activitiesModel.SupervisorPlanned
	byStaff      map[int64][]*activitiesModel.SupervisorPlanned
	priorByStaff map[int64][]*activitiesModel.SupervisorPlanned
	existing     map[instanceStudentPair]*scheduleModel.InstanceStaff
	created      int
	removed      int
	repointed    int
}

// reconcilePredecessorInstanceStaff aligns schedule.instance_staff on ONE
// template's already-materialized, still-planned occurrences with the
// template's current activities.supervisors rows, restricted to the given
// staff — the supervisor twin of ReconcileSourcedTemplateRosters. Rows
// recording a real decision (a substitution, an absence, a sick-cascade
// stamp) are never touched, and a staff member a planner had removed from one
// occurrence by hand is not resurrected (priorSupervisors semantics).
func (s *TemplateService) reconcilePredecessorInstanceStaff(
	ctx context.Context,
	templateID int64,
	staffIDs []int64,
	from timezone.Date,
	priorSupervisors []*activitiesModel.SupervisorPlanned,
) error {
	if templateID <= 0 || len(staffIDs) == 0 {
		return nil
	}
	if today := s.deps.Today(); from.Before(today) {
		from = today
	}
	instances, instanceIDs, err := loadPlannedTemplateInstances(ctx, s.deps.ActivityInstanceRepo, templateID, from, "reconcile predecessor staff")
	if err != nil || len(instances) == 0 {
		return err
	}
	pass, err := s.loadInstanceStaffReconciliation(ctx, templateID, instanceIDs, priorSupervisors)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if err := s.reconcileOccurrenceStaff(ctx, inst, staffIDs, pass); err != nil {
			return err
		}
	}
	if pass.created > 0 || pass.removed > 0 || pass.repointed > 0 {
		s.getLogger().Info("reconciled materialized staff after series roster change",
			slog.Int64("template_id", templateID),
			slog.Int("staff", len(staffIDs)),
			slog.Int("rows_created", pass.created),
			slog.Int("rows_removed", pass.removed),
			slog.Int("rows_repointed", pass.repointed),
		)
	}
	return nil
}

func (s *TemplateService) loadInstanceStaffReconciliation(
	ctx context.Context,
	templateID int64,
	instanceIDs []int64,
	priorSupervisors []*activitiesModel.SupervisorPlanned,
) (*instanceStaffReconciliation, error) {
	supervisors, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, &ScheduleError{Op: "reconcile predecessor staff: load supervisors", Err: err}
	}
	existingRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "reconcile predecessor staff: load existing rows", Err: err}
	}
	pass := &instanceStaffReconciliation{
		supervisors:  supervisors,
		byStaff:      supervisorsByStaff(supervisors),
		priorByStaff: supervisorsByStaff(priorSupervisors),
		existing:     make(map[instanceStudentPair]*scheduleModel.InstanceStaff, len(existingRows)),
	}
	for _, row := range existingRows {
		pass.existing[instanceStudentPair{instanceID: row.InstanceID, studentID: row.StaffID}] = row
	}
	return pass, nil
}

func supervisorsByStaff(rows []*activitiesModel.SupervisorPlanned) map[int64][]*activitiesModel.SupervisorPlanned {
	byStaff := make(map[int64][]*activitiesModel.SupervisorPlanned, len(rows))
	for _, sup := range rows {
		byStaff[sup.StaffID] = append(byStaff[sup.StaffID], sup)
	}
	return byStaff
}

// reconcileOccurrenceStaff aligns one occurrence's staff rows for the given
// staff members.
func (s *TemplateService) reconcileOccurrenceStaff(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
	staffIDs []int64,
	pass *instanceStaffReconciliation,
) error {
	periodID := calendarPeriodID(inst)
	instanceDate := timezone.Date(inst.Date)
	primaryStaffID, hasPrimary := effectivePrimarySupervisor(pass.supervisors, instanceDate, periodID)
	for _, staffID := range staffIDs {
		desired := supervisorsPlanStaffOn(pass.byStaff[staffID], instanceDate, periodID)
		key := instanceStudentPair{instanceID: inst.ID, studentID: staffID}
		row, exists := pass.existing[key]
		wantPrimary := hasPrimary && staffID == primaryStaffID
		var err error
		switch {
		case desired && !exists:
			// The pre-edit rows already planned this occurrence and the row is
			// gone regardless — a hand removal that stays.
			if !supervisorsPlanStaffOn(pass.priorByStaff[staffID], instanceDate, periodID) {
				err = s.addOccurrenceStaff(ctx, key, wantPrimary, pass)
			}
		case desired && exists && instanceStaffRowIsStillPlanned(row):
			err = s.repointOccurrencePrimary(ctx, row, wantPrimary, pass)
		case !desired && exists && instanceStaffRowIsStillPlanned(row):
			err = s.removeOccurrenceStaff(ctx, key, row, pass)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *TemplateService) addOccurrenceStaff(ctx context.Context, key instanceStudentPair, isPrimary bool, pass *instanceStaffReconciliation) error {
	fresh := &scheduleModel.InstanceStaff{
		InstanceID: key.instanceID,
		StaffID:    key.studentID,
		IsPrimary:  isPrimary,
	}
	if err := s.deps.InstanceStaffRepo.Create(ctx, fresh); err != nil {
		return &ScheduleError{Op: "reconcile predecessor staff: add staff", Err: err}
	}
	pass.existing[key] = fresh
	pass.created++
	return nil
}

// repointOccurrencePrimary moves the occurrence's Hauptbetreuung. A moved
// lead changes no membership, so without it the occurrence kept pointing at
// the old lead while the template rows already named the new one (#2187
// review).
func (s *TemplateService) repointOccurrencePrimary(ctx context.Context, row *scheduleModel.InstanceStaff, wantPrimary bool, pass *instanceStaffReconciliation) error {
	if row.IsPrimary == wantPrimary {
		return nil
	}
	row.IsPrimary = wantPrimary
	if _, err := s.deps.InstanceStaffRepo.UpdateColumns(ctx, row, "is_primary"); err != nil {
		return &ScheduleError{Op: "reconcile predecessor staff: update primary", Err: err}
	}
	pass.repointed++
	return nil
}

func (s *TemplateService) removeOccurrenceStaff(ctx context.Context, key instanceStudentPair, row *scheduleModel.InstanceStaff, pass *instanceStaffReconciliation) error {
	if err := s.deps.InstanceStaffRepo.Delete(ctx, row.ID); err != nil {
		return &ScheduleError{Op: "reconcile predecessor staff: remove staff", Err: err}
	}
	delete(pass.existing, key)
	pass.removed++
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
		if !validityWindowsOverlap(timezone.Date(row.ValidFrom), timezoneDatePtr(row.ValidUntil), rowFrom, rowUntil) {
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
