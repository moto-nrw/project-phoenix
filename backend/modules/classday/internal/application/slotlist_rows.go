package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// enrichEntries filters by source/read access, resolves names/classes/groups,
// and maps to display rows.
func (s *service) enrichEntries(ctx context.Context, entries []mergedEntry, source classday.Source, access studentReadAccess, date calendar.Date) ([]classday.Row, error) {
	// matchesSource is the per-source selection predicate. A Plan list selects
	// rows that are planned, an Ist list rows that are present, an Abgleich keeps
	// everything. It is applied once to build `filtered` and re-applied after the
	// enrollment gate below mutates entries, so the two never diverge.
	matchesSource := func(e mergedEntry) bool {
		switch source {
		case classday.SourcePlanned:
			return e.Planned
		case classday.SourceActual:
			return e.Present
		case classday.SourceReconciliation:
			return true
		default:
			return false
		}
	}
	filtered := make([]mergedEntry, 0, len(entries))
	for _, entry := range entries {
		if matchesSource(entry) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		return []classday.Row{}, nil
	}

	studentIDSet := map[int64]struct{}{}
	studentIDs := []int64{}
	for _, entry := range filtered {
		if _, ok := studentIDSet[entry.StudentID]; !ok {
			studentIDSet[entry.StudentID] = struct{}{}
			studentIDs = append(studentIDs, entry.StudentID)
		}
	}
	studentRows, err := s.students.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	students := make(map[int64]*peopledirectory.Student, len(studentRows))
	for i := range studentRows {
		students[studentRows[i].ID] = &studentRows[i]
	}

	// Date-effective enrollment gate: the lifecycle scheduler inactivates a
	// student by flipping users.students.status only — already-materialized
	// schedule.instance_students rows survive. A *planned* row for a child not
	// enrolled on the requested date must be dropped, or it surfaces as an
	// unexplained "Fehlt" (#1565 review). Documented presence is ground truth
	// and always kept: a real visit/attendance record outranks the enrollment
	// interval, so present rows pass regardless — but the enrollment check has
	// established that the roster's *expectation* is invalid, so a present row
	// for a non-enrolled child must shed its planned classification too, or the
	// Abgleich reports it as planned-and-present and inflates the planned
	// counter. Keep the presence, clear the plan, so it reads as "ungeplant
	// anwesend" (#1565 review pass 2).
	today := s.todayDate()
	enrolled := make([]mergedEntry, 0, len(filtered))
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil {
			continue
		}
		enrolledOn, err := eligibleOn(s.rules, student, date, today, false)
		if err != nil {
			return nil, err
		}
		if !enrolledOn {
			if !entry.Present {
				continue
			}
			entry.Planned = false
			entry.PlannedStatus = ""
			entry.PlannedSubstatus = nil
			// Clearing the plan can drop the entry below the source predicate it
			// originally satisfied. On a Plan list the row only reached `filtered`
			// because it was planned; now de-planned, keeping it would leak an
			// "ungeplant" row into the planned roster — rendered as Geplant yet
			// excluded from the planned counter (a contradictory row). Re-apply the
			// source filter so a de-planned present row survives only where the list
			// actually selects on presence (Ist/Abgleich), not on a Plan list
			// (#1565 review pass 2).
			if !matchesSource(entry) {
				continue
			}
		}
		enrolled = append(enrolled, entry)
	}
	filtered = enrolled
	if len(filtered) == 0 {
		return []classday.Row{}, nil
	}

	readableStudents := make(map[int64]*peopledirectory.Student, len(students))
	personIDs := make([]int64, 0, len(students))
	groupIDSet := map[int64]struct{}{}
	groupIDs := []int64{}
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil || !access.canReadGroup(student.GroupID) {
			continue
		}
		if _, ok := readableStudents[student.ID]; ok {
			continue
		}
		readableStudents[student.ID] = student
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil {
			if _, ok := groupIDSet[*student.GroupID]; !ok {
				groupIDSet[*student.GroupID] = struct{}{}
				groupIDs = append(groupIDs, *student.GroupID)
			}
		}
	}
	if len(readableStudents) == 0 {
		return []classday.Row{}, nil
	}
	personRows, err := s.persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	persons := make(map[int64]*peopledirectory.Person, len(personRows))
	for i := range personRows {
		persons[personRows[i].ID] = &personRows[i]
	}
	groups := map[int64]schoolstructure.Group{}
	if len(groupIDs) > 0 {
		groupRows, err := s.groups.ListGroupsByID(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
		for _, group := range groupRows {
			groups[group.ID] = group
		}
	}

	rows := make([]classday.Row, 0, len(filtered))
	for _, entry := range filtered {
		student := readableStudents[entry.StudentID]
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		groupName := ""
		if student.GroupID != nil {
			if group, ok := groups[*student.GroupID]; ok {
				groupName = group.Name
			}
		}
		rows = append(rows, classday.Row{
			StudentID:   entry.StudentID,
			Name:        person.FullName(),
			SchoolClass: student.SchoolClass,
			GroupName:   groupName,
			GroupID:     student.GroupID,
			InstanceID:  entry.InstanceID,
			Slot:        entry.SlotLabel,
			RoomName:    entry.RoomName,
			PickupTime:  entry.PickupTime,
			Planned:     entry.Planned,
			Present:     entry.Present,
			Unplanned:   entry.Present && !entry.Planned,
			Excused:     entry.Planned && !entry.Present && signedOffAbsence(entry),
			StatusLabel: statusLabel(entry, source),
		})
	}
	return rows, nil
}

// signedOffAbsence reports whether an absent row carries explicit evidence
// that the absence was registered — sick, excused, a field/class trip, or a
// cancelled care day. A merely non-empty substatus is NOT proof: a `late`
// context (a valid state that can linger after a status change) or `other`
// says nothing about a genuine sign-off. Treating those as "Abgemeldet" would
// move a genuinely missing child out of the Missing counter and suppress a
// safety-relevant missing-child signal, so anything short of registered
// absence evidence falls through to "Fehlt".
func signedOffAbsence(entry mergedEntry) bool {
	if entry.PlannedStatus != ports.AttendanceAbsent || entry.PlannedSubstatus == nil {
		return false
	}
	switch *entry.PlannedSubstatus {
	case substatusSick, // instance substatus / status day
		substatusExcused,                   // instance substatus / status day / partial-day excusal
		substatusFieldTrip,                 // instance substatus
		careplan.StudentStatusDayClassTrip, // status day
		string(ports.CareDayCancelled):     // cancelled care day
		return true
	default:
		return false
	}
}

// partialCoversPickup reports whether a partial-day excusal cutoff is at or
// before the child's cohort pickup time. Mirrors ApplyPartialAbsence's
// start_time >= excused_from predicate for slots: earlier pickups remain
// expected; pickups at/after the cutoff are signed off.
func partialCoversPickup(pickupHHMM, excusedFromHHMM string) bool {
	if pickupHHMM == "" || excusedFromHHMM == "" {
		return false
	}
	return pickupHHMM >= excusedFromHHMM
}

// voidPlanRegisteredAbsence reports whether a deferred (not-yet-started) slot's
// roster row is a registered all-day sign-off that must survive the void plan as
// "Abgemeldet" rather than be suppressed like an unexplained not-yet-due no-show.
// It returns the substatus to stamp on the retained entry. Two shapes qualify,
// exactly the ones the normal merge and the pickup cohort render as "Abgemeldet":
//
//   - A status-day sick/excused/class-trip stamp on the row itself: ApplyStatusDay
//     writes status=absent plus the substatus (class trip → "field_trip") and a
//     student_status_day_id, so the evidence rides the row even before the slot
//     starts.
//   - A cancelled care day ("Kommt heute nicht"), which lives in the timeless
//     arrival/pickup exceptions rather than on the row. A manual override
//     (ManualStatusAt) still wins and is excluded here, exactly as the normal
//     merge's cancelled-care-day branch treats it.
//
// Anything else — an expected no-show, a lifecycle absent without a sign-off
// substatus, a not_scheduled non-booking — is not a registered absence and stays
// suppressed so the deferred slot never prints a not-yet-due child as "Fehlt".
func voidPlanRegisteredAbsence(row ports.InstanceStudent, careDay ports.CareDay) (string, bool) {
	// The care plan never booked this child into the OGS on this weekday
	// (CareDayNotScheduled means a plan exists but does not cover today), yet a
	// broad sick/excused/class-trip status day can still stamp the roster row
	// absent+substatus for a day the child was never expected. On a deferred
	// (not-yet-started) slot that is not a registered absence — the child does
	// not belong on the list at all — so drop it here exactly as the normal
	// merge does at its CareDayNotScheduled branch (classifyPlannedRow); keeping it
	// would inflate the planned and excused lists with unbooked children. A
	// manual per-slot override (ManualStatusAt) is a human decision that wins and
	// is deliberately excluded from this guard, matching that same merge branch.
	// A child with no plan at all resolves to CareDayUnknown (not NotScheduled),
	// so a genuine sign-off on an unplanned child still survives below (#1565
	// review pass 10).
	if row.ManualStatusAt == nil && careDay == ports.CareDayNotScheduled {
		return "", false
	}
	if row.Status == ports.AttendanceAbsent && row.Substatus != nil {
		switch *row.Substatus {
		case substatusSick,
			substatusExcused,
			substatusFieldTrip:
			return *row.Substatus, true
		}
	}
	if row.ManualStatusAt == nil && careDay == ports.CareDayCancelled {
		return string(ports.CareDayCancelled), true
	}
	return "", false
}

func statusLabel(entry mergedEntry, source classday.Source) string {
	switch source {
	case classday.SourcePlanned:
		return "Geplant"
	case classday.SourceActual:
		return "Anwesend"
	case classday.SourceReconciliation:
		switch {
		case entry.Present && !entry.Planned:
			return "Ungeplant anwesend"
		case entry.Present:
			return "Anwesend"
		case signedOffAbsence(entry):
			return "Abgemeldet (entschuldigt)"
		default:
			return "Fehlt"
		}
	default:
		return ""
	}
}

func countRows(rows []classday.Row, source classday.Source) classday.Counters {
	// A slot list may include several offerings and the same child can hold one
	// row in each, but these counters are child headcounts — the export subtitle
	// labels them "geplante Kinder" / "anwesende Kinder", not assignment entries.
	// Deduplicate by StudentID per category so a child assigned to two offerings
	// counts once, never twice (#1565 review pass 1 P2). Categories stay
	// independent (as the per-row logic already was): a child present in one slot
	// and missing in another appears in both headcounts, matching the row logic.
	planned := map[int64]struct{}{}
	present := map[int64]struct{}{}
	missing := map[int64]struct{}{}
	excused := map[int64]struct{}{}
	unplanned := map[int64]struct{}{}
	for _, row := range rows {
		if row.Planned {
			planned[row.StudentID] = struct{}{}
		}
		if row.Present {
			present[row.StudentID] = struct{}{}
		}
		switch {
		case row.Planned && !row.Present && row.Excused:
			// Registered sign-off: a justified absence, not an unexplained gap.
			excused[row.StudentID] = struct{}{}
		case row.Planned && !row.Present:
			missing[row.StudentID] = struct{}{}
		}
		if row.Unplanned {
			unplanned[row.StudentID] = struct{}{}
		}
	}
	counters := classday.Counters{
		Planned:   len(planned),
		Present:   len(present),
		Missing:   len(missing),
		Excused:   len(excused),
		Unplanned: len(unplanned),
	}
	// Planned/actual previews intentionally show only their own counter;
	// the merge counters are a reconciliation concept.
	switch source {
	case classday.SourcePlanned:
		return classday.Counters{Planned: counters.Planned}
	case classday.SourceActual:
		return classday.Counters{Present: counters.Present}
	default:
		return counters
	}
}
