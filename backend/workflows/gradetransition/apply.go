package gradetransition

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// Apply executes the transition, refusing with ErrPreviewStale when
// expectedFingerprint is set and no longer matches the cohort resolved under
// the locks. An empty fingerprint skips that check (a caller without a
// preview in hand).
func (w *Workflow) Apply(ctx context.Context, id int64, expectedFingerprint string) (result Result, err error) {
	err = w.run(ctx, "apply", OperationApply, id, func(txCtx context.Context, actor Actor) error {
		if err := w.lockGates(txCtx); err != nil {
			return err
		}
		transition, err := w.deps.Structure.FindTransition(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		if err := validateCanApply(transition); err != nil {
			return err
		}
		applied, err := w.executeApply(txCtx, actor, transition, expectedFingerprint)
		if err != nil {
			return err
		}
		result = applied
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// lockGates takes the three tenant-wide gates an apply or revert needs, in
// the project-wide order: the student class-writes gate FIRST, then the
// recurrence gate, then the grade transition gate.
//
// The class-writes gate comes first because an ordinary request can also
// hold it: a create-student request takes it shared and may afterwards touch
// recurrence-derived state, so an apply holding the recurrence gate and then
// asking for the class-writes gate would close the cycle. Holding it
// exclusively for the rest of the transaction also shuts the one window the
// row locks cannot reach: a child created in a mapped class, or moved in
// from an unmapped one, has no row to lock, so it either lands before the
// cohort re-read (and is refused as a late arrival) or waits for the commit.
//
// The recurrence gate precedes the transition gate because both operations
// mutate recurrence-derived roster state: a re-plan holds the recurrence
// gate, deletes planned instances (locking their roster rows) and only then
// asks for the transition gate. The materializer takes the same two gates in
// the same order, so a materialization pass can neither insert an upcoming
// roster row for a child this apply is about to graduate nor race the
// reconciliation.
func (w *Workflow) lockGates(ctx context.Context) error {
	if err := w.deps.Directory.LockEnrollmentClassWritesExclusive(ctx); err != nil {
		return err
	}
	if err := w.deps.LockRecurrenceWrites(ctx); err != nil {
		return err
	}
	return w.deps.Structure.LockTransitions(ctx)
}

// validateCanApply wraps every refusal in ErrTransitionNotDraft: they are
// all the same stale-state outcome (the draft the admin loaded has since
// been applied, reverted or emptied).
func validateCanApply(transition schoolstructure.Transition) error {
	if transition.CanApply() {
		return nil
	}
	if transition.IsApplied() {
		return fmt.Errorf("%w: transition has already been applied", ErrTransitionNotDraft)
	}
	if transition.IsReverted() {
		return fmt.Errorf("%w: transition has been reverted", ErrTransitionNotDraft)
	}
	return fmt.Errorf("cannot apply transition: %w with mappings", ErrTransitionNotDraft)
}

// executeApply runs the owner commands in the cutover order. Every step is a
// reversible row change inside the caller's transaction.
func (w *Workflow) executeApply(ctx context.Context, actor Actor, transition schoolstructure.Transition, expectedFingerprint string) (Result, error) {
	result := Result{TransitionID: transition.ID, Warnings: make([]string, 0)}
	promoteClasses, graduateClasses := categorizeMappings(transition.Mappings)

	// Resolve BOTH definitive cohorts ONCE and hold a FOR UPDATE lock on
	// every row. The check-in guard, the history snapshot and the class and
	// status writes all operate on exactly these locked sets.
	promotions, graduates, err := w.lockCohorts(ctx, promoteClasses, graduateClasses)
	if err != nil {
		return result, err
	}

	// The admin confirmed a specific preview. Refuse if the locked cohort is
	// no longer the one that preview described.
	if err := ensureFingerprintMatches(expectedFingerprint, transition.Mappings, promotions, graduates); err != nil {
		return result, err
	}

	// A graduating child who is still checked in would be stranded: the row
	// flips to alumnus with an open record the kiosk can no longer close.
	if err := w.ensureGraduatesNotCheckedIn(ctx, graduates); err != nil {
		return result, err
	}

	// Free the physical bracelets BEFORE the history write, so the ledger
	// row carries the tag that was actually released. A graduate keeps their
	// person row (graduation is a soft delete) and would otherwise keep
	// holding a tag no staff-facing route can release.
	releasedTags, err := w.releaseGraduateTags(ctx, graduates)
	if err != nil {
		return result, err
	}

	if err := w.recordHistory(ctx, transition, promotions, graduates, releasedTags); err != nil {
		return result, err
	}

	// Graduate BEFORE promoting: a promotion can move children into a class
	// that graduates in the same transition, and the graduation writes must
	// never see a row the promotion already moved.
	if len(graduates) > 0 {
		graduated, err := w.deps.Directory.GraduateStudents(ctx, studentIDsOf(graduates))
		if err != nil {
			return result, fmt.Errorf("failed to graduate students: %w", err)
		}
		result.StudentsGraduated = int(graduated)
	}
	promoted, err := w.applyPromotions(ctx, transition.Mappings, promotions)
	if err != nil {
		return result, err
	}
	result.StudentsPromoted = promoted

	// The Klassenlehrer assignments and the class-list entries follow the
	// same renames as the student rows; every rewrite lands in the
	// transition's ledgers so the revert can replay it exactly.
	if err := w.remapClassTeacherAssignments(ctx, transition.ID, transition.Mappings); err != nil {
		return result, err
	}
	if err := w.remapClassListEntries(ctx, actor, transition.ID, transition.Mappings); err != nil {
		return result, err
	}

	// Drop the departed children from already-materialized rosters,
	// archiving each removed row for the revert. MUST run before the
	// offering-source resync: the resync would remove the same rows plainly,
	// leaving this pass nothing to archive.
	if len(graduates) > 0 {
		if err := w.deps.Rosters.RemoveStudentsFromFutureRosters(ctx, transition.ID, studentIDsOf(graduates)); err != nil {
			return result, fmt.Errorf("failed to reconcile graduated rosters: %w", err)
		}
	}

	// The class writes changed Jahrgänge: re-reconcile every offering-sourced
	// template so its grade-filtered roster follows the children. Runs under
	// the recurrence gate inside the same transaction.
	if err := w.deps.ResyncOfferingRosters(ctx, w.deps.Today()); err != nil {
		return result, fmt.Errorf("failed to resync offering-sourced templates: %w", err)
	}

	// Record which instances already existed, AFTER the archive pass and
	// while the gates still exclude every materializer. Everything inserted
	// above this marker is built during the alumnus window and is the
	// revert's to refill.
	baseline, err := w.deps.Rosters.CurrentRosterBaseline(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to record roster baseline: %w", err)
	}
	if err := w.deps.Structure.MarkTransitionApplied(ctx, transition.ID, actor.AccountID, w.deps.Now(), &baseline); err != nil {
		if errors.Is(err, schoolstructure.ErrTransitionStateConflict) {
			return result, fmt.Errorf("%w: transition has already been applied", ErrTransitionNotDraft)
		}
		return result, fmt.Errorf("failed to update transition status: %w", translateStructureError(err))
	}

	result.Status = schoolstructure.TransitionStatusApplied
	result.CanRevert = true
	if result.StudentsGraduated > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d students were marked as alumni and hidden from the app (graduates)", result.StudentsGraduated))
	}
	return result, nil
}

func categorizeMappings(mappings []schoolstructure.TransitionMapping) (promote, graduate []string) {
	for _, mapping := range mappings {
		if mapping.IsGraduating() {
			graduate = append(graduate, mapping.FromClass)
		} else {
			promote = append(promote, mapping.FromClass)
		}
	}
	return promote, graduate
}

// lockCohorts resolves the promoting and graduating cohorts, takes a FOR
// UPDATE lock on each surviving row in ONE ascending-id pass (the
// deadlock-safe order every other student-row locker uses), and
// re-validates every row UNDER its lock: the locked row decides which cohort
// the child belongs to and which status the history records. A child moved
// out of every mapped class in between is dropped, one already turned
// alumnus is skipped, and a child that ENTERED a mapped class behind the
// snapshot is refused as a late arrival.
func (w *Workflow) lockCohorts(ctx context.Context, promoteClasses, graduateClasses []string) (promotions, graduates []cohortStudent, err error) {
	classes := make([]string, 0, len(promoteClasses)+len(graduateClasses))
	classes = append(classes, promoteClasses...)
	classes = append(classes, graduateClasses...)
	if len(classes) == 0 {
		return nil, nil, nil
	}
	snapshot, err := w.cohortOf(ctx, classes)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(snapshot, func(i, j int) bool { return snapshot[i].StudentID < snapshot[j].StudentID })
	graduateSet, promoteSet := classSet(graduateClasses), classSet(promoteClasses)
	locked := make(map[int64]bool, len(snapshot))
	for _, student := range snapshot {
		locked[student.StudentID] = true
		record, err := w.deps.Directory.ReadEnrollmentStudent(ctx, student.StudentID, "update")
		if errors.Is(err, peopledirectory.ErrStudentNotFound) {
			continue // the row vanished before it could be locked
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to lock transition student %d: %w", student.StudentID, err)
		}
		if record.Status == peopledirectory.StudentStatusAlumnus {
			continue
		}
		current := cohortStudent{StudentID: record.ID, PersonID: record.PersonID, SchoolClass: record.SchoolClass, Status: record.Status}
		switch {
		case graduateSet[current.SchoolClass]:
			graduates = append(graduates, current)
		case promoteSet[current.SchoolClass]:
			promotions = append(promotions, current)
		}
	}
	// Re-read UNDER the locks: this statement sees every concurrent commit
	// that landed since the snapshot, and the exclusive class-writes gate
	// means no further arrival can commit behind it.
	current, err := w.cohortOf(ctx, classes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to re-check transition students: %w", err)
	}
	for _, student := range current {
		if !locked[student.StudentID] {
			return nil, nil, ErrPreviewStale
		}
	}
	return promotions, graduates, nil
}

func classSet(classes []string) map[string]bool {
	set := make(map[string]bool, len(classes))
	for _, class := range classes {
		set[class] = true
	}
	return set
}

func studentIDsOf(cohort []cohortStudent) []int64 {
	ids := make([]int64, 0, len(cohort))
	for _, student := range cohort {
		ids = append(ids, student.StudentID)
	}
	return ids
}

// ensureGraduatesNotCheckedIn refuses the apply when any of the (already
// locked) graduating children is currently checked in: an open visit or an
// open attendance record for today. A concurrent check-in either committed
// before the row lock (and is observed here) or blocks until this apply
// commits and then re-reads the alumnus status.
func (w *Workflow) ensureGraduatesNotCheckedIn(ctx context.Context, graduates []cohortStudent) error {
	if len(graduates) == 0 {
		return nil
	}
	ids := studentIDsOf(graduates)
	checkedIn := make(map[int64]struct{})
	visits, err := w.deps.Presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: ids, OpenOnly: true})
	if err != nil {
		return fmt.Errorf("failed to check active visits: %w", err)
	}
	for _, visit := range visits {
		checkedIn[visit.StudentID] = struct{}{}
	}
	today := w.deps.Today()
	attendance, err := w.deps.Presence.ListAttendance(ctx, studentpresence.AttendanceFilter{
		StudentIDs: ids, FromDate: today, UntilDate: today, NewestFirst: true, StudentOrder: true,
	})
	if err != nil {
		return fmt.Errorf("failed to check attendance: %w", err)
	}
	seen := make(map[int64]bool, len(ids))
	for _, row := range attendance {
		if seen[row.StudentID] {
			continue
		}
		seen[row.StudentID] = true
		if row.CheckOutTime == nil {
			checkedIn[row.StudentID] = struct{}{}
		}
	}
	if len(checkedIn) > 0 {
		return fmt.Errorf("%w: %d student(s) must be checked out first", ErrGraduatesCheckedIn, len(checkedIn))
	}
	return nil
}

// releaseGraduateTags clears the bracelet of every graduating child through
// the People Directory and returns what each was holding, keyed by student.
func (w *Workflow) releaseGraduateTags(ctx context.Context, graduates []cohortStudent) (map[int64]string, error) {
	if len(graduates) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(graduates))
	studentsByPerson := make(map[int64]int64, len(graduates))
	for _, student := range graduates {
		personIDs = append(personIDs, student.PersonID)
		studentsByPerson[student.PersonID] = student.StudentID
	}
	released, err := w.deps.Directory.ReleaseTags(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to release graduate RFID tags: %w", err)
	}
	result := make(map[int64]string, len(released))
	for _, entry := range released {
		if studentID, ok := studentsByPerson[entry.PersonID]; ok && entry.TagID != "" {
			result[studentID] = entry.TagID
		}
	}
	return result, nil
}

// recordHistory writes one ledger row per affected child. Both cohorts are
// the locked, re-validated sets, so the rows recorded are exactly the rows
// promoted and graduated.
func (w *Workflow) recordHistory(ctx context.Context, transition schoolstructure.Transition, promotions, graduates []cohortStudent, releasedTags map[int64]string) error {
	students := make([]cohortStudent, 0, len(promotions)+len(graduates))
	students = append(students, promotions...)
	students = append(students, graduates...)
	if len(students) == 0 {
		return nil
	}
	names, err := w.deps.Directory.ListStudentNamesByID(ctx, studentIDsOf(students))
	if err != nil {
		return fmt.Errorf("failed to resolve transition student names: %w", err)
	}
	nameByStudent := make(map[int64]string, len(names))
	for _, name := range names {
		nameByStudent[name.StudentID] = peopledirectory.Person{FirstName: name.FirstName, LastName: name.LastName}.FullName()
	}
	targets := classTargets(transition.Mappings)
	entries := make([]schoolstructure.TransitionHistoryEntry, 0, len(students))
	for _, student := range students {
		toClass := targets[student.SchoolClass]
		action := schoolstructure.TransitionActionPromoted
		if toClass == nil {
			action = schoolstructure.TransitionActionGraduated
		}
		name := nameByStudent[student.StudentID]
		if name == "" || name == " " {
			name = PurgedStudentName
		}
		fromStatus := student.Status
		entry := schoolstructure.TransitionHistoryEntry{
			TransitionID: transition.ID, StudentID: student.StudentID, PersonName: name,
			FromClass: student.SchoolClass, ToClass: toClass, Action: action, FromStatus: &fromStatus,
		}
		if tag, ok := releasedTags[student.StudentID]; ok && tag != "" {
			released := tag
			entry.RFIDTag = &released
		}
		entries = append(entries, entry)
	}
	if err := w.deps.Structure.AppendTransitionHistory(ctx, entries); err != nil {
		return fmt.Errorf("failed to create history: %w", translateStructureError(err))
	}
	return nil
}

func classTargets(mappings []schoolstructure.TransitionMapping) map[string]*string {
	targets := make(map[string]*string, len(mappings))
	for _, mapping := range mappings {
		targets[mapping.FromClass] = mapping.ToClass
	}
	return targets
}

// applyPromotions moves exactly the locked, history-recorded promotion
// cohort into its mapped class, one guarded command per from-class in
// deterministic order.
func (w *Workflow) applyPromotions(ctx context.Context, mappings []schoolstructure.TransitionMapping, promotions []cohortStudent) (int, error) {
	if len(promotions) == 0 {
		return 0, nil
	}
	targets := classTargets(mappings)
	byFromClass := make(map[string][]int64)
	for _, student := range promotions {
		target := targets[student.SchoolClass]
		if target == nil || *target == "" {
			continue
		}
		byFromClass[student.SchoolClass] = append(byFromClass[student.SchoolClass], student.StudentID)
	}
	fromClasses := make([]string, 0, len(byFromClass))
	for fromClass := range byFromClass {
		fromClasses = append(fromClasses, fromClass)
	}
	sort.Strings(fromClasses)
	var promoted int64
	for _, fromClass := range fromClasses {
		count, err := w.deps.Directory.PromoteStudents(ctx, byFromClass[fromClass], fromClass, *targets[fromClass])
		if err != nil {
			return 0, fmt.Errorf("failed to promote students: %w", err)
		}
		promoted += count
	}
	return int(promoted), nil
}
