package application

import (
	"context"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentWrite is one create or update of a child, as the caller submits it:
// the owned row plus the departure plan it carries. A nil plan field means
// "not supplied", which is what keeps a write that touches unrelated columns
// from rewriting the stored plan.
type StudentWrite struct {
	Record domain.StudentRecord
	Plan   domain.DeparturePlan
	// Baseline is the plan the caller's read hydrated, when it had one. It is
	// what tells a field the caller really changed from one that merely rode
	// along on that read.
	Baseline *domain.DeparturePlan
	// CompanionNote is the free-text "mit wem" detail. nil leaves the stored
	// note alone unless the resolved plan no longer allows an accompanied day.
	CompanionNote *string
	// NoteSupplied distinguishes "the caller cleared the note" from "the caller
	// said nothing about it", which a nil pointer alone cannot.
	NoteSupplied bool
}

// A failed composite command must undo its own writes even when an ambient
// transaction catches the error and continues with unrelated work.
func (s *StudentService) runStudentWrite(ctx context.Context, write func(context.Context) error) error {
	return s.tx.RunWrite(ctx, func(txCtx context.Context) error {
		return s.tx.RunSavepoint(txCtx, write)
	})
}

// CreateStudent inserts a child and writes its departure plan.
//
// The shared class-writes gate comes first: a brand-new row is exactly the
// arrival a grade transition cannot row-lock against, so the insert has to wait
// out a running apply rather than landing in a class it has already emptied.
func (s *StudentService) CreateStudent(ctx context.Context, write StudentWrite) (result domain.StudentRecord, err error) {
	err = s.run(ctx, "create_student", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}

		// No stored plan to resolve against on a create.
		resolved := write.Plan.Align(nil)
		if err := domain.ValidateStudentRecord(write.Record, write.Plan, resolved, write.CompanionNote, nil); err != nil {
			return err
		}

		record, insertStats, err := s.store.InsertRecord(txCtx, write.Record)
		stats.Add(insertStats)
		if err != nil {
			return err
		}
		membershipID, err := s.owners.Enroll(txCtx, record)
		if err != nil {
			return err
		}
		err = s.owners.SaveCare(txCtx, membershipID, record, resolved, noteToStore(resolved, write.CompanionNote), write.Plan.Touched() || write.NoteSupplied)
		if err != nil {
			return err
		}
		result = applyPlanToRecord(record, resolved, noteToStore(resolved, write.CompanionNote))
		return nil
	})
	return result, err
}

// UpdateStudent rewrites a child and reconciles everything that depends on its
// departure plan, in the one order that is safe.
//
//  1. the shared class-writes gate, before any row lock, so the acquisition
//     order is gate-then-rows for every writer and cannot cycle against a grade
//     transition holding the gate exclusively;
//  2. the subject's row lock, before reading its stored plan or its links, so
//     both reads see the state this write actually overwrites;
//  3. rebase the fields the caller never touched onto that freshly locked state;
//  4. align the plan so the validation judges what will be persisted;
//  5. reconcile the links the resolved plan no longer allows, refusing when a
//     far child would be left without a "mit wem" detail;
//  6. validate, write the row, write the plan;
//  7. drop the trimmed links last, so a failure never leaves links deleted for a
//     plan that was never stored.
func (s *StudentService) UpdateStudent(ctx context.Context, write StudentWrite) (result domain.StudentRecord, err error) {
	err = s.run(ctx, "update_student", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		if err := s.lockSubjectForWrite(txCtx, stats, write); err != nil {
			return err
		}

		stored, found, planStats, err := s.store.FindDeparturePlan(txCtx, write.Record.ID)
		stats.Add(planStats)
		if err != nil {
			return err
		}
		var current *domain.DeparturePlan
		if found {
			current = &stored
		}

		plan := write.Plan.Rebase(write.Baseline, current)
		resolved := plan.Align(current)

		trim, err := s.reconcileCompanions(txCtx, stats, write.Record.ID, plan, resolved)
		if err != nil {
			return err
		}

		// A structured link answers the accompanied-requires-a-note invariant
		// just like the free-text note does, but it lives in another owner's
		// table and is not part of the row. Derive it here, the one layer every
		// update passes through, so a caller that knows nothing about links can
		// still save a child whose "mit wem" is answered by one.
		var covered map[string]bool
		if trim != nil {
			covered = trim.KeptDays
		}
		if err := domain.ValidateStudentRecord(write.Record, plan, resolved, write.CompanionNote, covered); err != nil {
			return err
		}

		record, updated, writeStats, err := s.store.UpdateRecord(txCtx, write.Record)
		stats.Add(writeStats)
		if err != nil {
			return err
		}
		if !updated {
			return domain.ErrStudentNotFound
		}

		note := noteToStore(resolved, write.CompanionNote)
		if err := s.saveStudentOwners(txCtx, record, resolved, note, plan.Touched() || write.NoteSupplied); err != nil {
			return err
		}

		if trim != nil && len(trim.DropIDs) > 0 {
			if err := s.companions.DeleteEdges(txCtx, trim.DropIDs); err != nil {
				return err
			}
		}
		result = applyPlanToRecord(record, resolved, note)
		return nil
	})
	return result, err
}

func (s *StudentService) saveStudentOwners(ctx context.Context, record domain.StudentRecord, plan domain.DeparturePlan, note *string, touched bool) error {
	membershipID, err := s.owners.Renew(ctx, record)
	if err != nil {
		return err
	}
	return s.owners.SaveCare(ctx, membershipID, record, plan, note, touched)
}

// DeleteStudent removes one child. The gate comes first for the same reason it
// does on every other student write.
func (s *StudentService) DeleteStudent(ctx context.Context, studentID int64) error {
	return s.run(ctx, "delete_student", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		deleted, deleteStats, err := s.store.DeleteRecord(txCtx, studentID)
		stats.Add(deleteStats)
		if err != nil {
			return err
		}
		if !deleted {
			return domain.ErrStudentNotFound
		}
		return nil
	})
}

// VerifyStrandingBatch decides the verdicts a coordinated multi-child write
// deferred, now against the state the whole batch leaves behind: every plan is
// written and every trimmed link is gone, so a child whose accompanied day went
// away in the same edit passes, while one genuinely left without an answer
// still fails. Nothing is excluded from the coverage read — unlike the per-write
// check, this runs after the deletions.
func (s *StudentService) VerifyStrandingBatch(ctx context.Context) error {
	batch := domain.StrandingBatchFromContext(ctx)
	if batch == nil {
		return nil
	}
	removed, removedDays := batch.Pending()
	if len(removed) == 0 {
		return nil
	}
	return s.run(ctx, "verify_student_stranding_batch", s.tx.RunRead,
		func(txCtx context.Context, stats *domain.OperationStats) error {
			// 0 excludes nobody: every link that still exists counts as cover.
			return s.decideStranding(txCtx, stats, 0, removed, removedDays)
		})
}

// lockSubjectForWrite takes the subject's row lock when this write will touch
// the departure columns — a plan was supplied, or a note was, which the plan
// write resolves against the stored plan and therefore rewrites the same
// columns. It is the first row lock of the transaction, which is what the
// companion far-end walk below assumes.
//
// A missing row is not an error here: the update that follows simply matches
// nothing and reports the not-found itself.
func (s *StudentService) lockSubjectForWrite(
	ctx context.Context,
	stats *domain.OperationStats,
	write StudentWrite,
) error {
	if write.Record.ID <= 0 || (!write.Plan.Touched() && !write.NoteSupplied) {
		return nil
	}
	_, _, lockStats, err := s.store.FindRecord(ctx, write.Record.ID, "UPDATE")
	stats.Add(lockStats)
	return err
}

// reconcileCompanions decides which links lose their basis under the plan this
// write is about to persist, and refuses when dropping one would strand the far
// child. It returns nil when it did not read the links at all — the plan was
// untouched, or it still allows every weekday, so no link can lose its basis.
func (s *StudentService) reconcileCompanions(
	ctx context.Context,
	stats *domain.OperationStats,
	studentID int64,
	plan, resolved domain.DeparturePlan,
) (*domain.CompanionTrim, error) {
	if studentID <= 0 || !plan.Touched() {
		return nil, nil
	}
	accompanied := resolved.AccompaniedDays()
	if len(accompanied) == len(domain.CompanionWeekdayKeys) {
		return nil, nil
	}
	if s.companions == nil {
		return nil, errors.New("people directory application: companion capability is not bound")
	}

	edges, err := s.companions.ListForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if len(edges) == 0 {
		return &domain.CompanionTrim{}, nil
	}

	trim := &domain.CompanionTrim{KeptDays: make(map[string]bool, len(domain.CompanionWeekdayKeys))}
	removedDays := make(map[int64][]string, len(edges))
	removed := make([]int64, 0, len(edges))
	for _, edge := range edges {
		far, ok := edge.Other(studentID)
		if !ok {
			continue
		}
		day := domain.CompanionWeekdayKeys[edge.Weekday]
		if accompanied[day] {
			trim.KeptDays[day] = true
			continue
		}
		trim.DropIDs = append(trim.DropIDs, edge.ID)
		if _, seen := removedDays[far]; !seen {
			removed = append(removed, far)
		}
		removedDays[far] = append(removedDays[far], day)
	}
	if len(trim.DropIDs) == 0 {
		return trim, nil
	}
	if err := s.lockCompanionFarEnds(ctx, stats, studentID, removed); err != nil {
		return nil, err
	}
	if batch := domain.StrandingBatchFromContext(ctx); batch != nil {
		for _, id := range removed {
			batch.Defer(id, removedDays[id])
		}
		return trim, nil
	}
	if err := s.decideStranding(ctx, stats, studentID, removed, removedDays); err != nil {
		return nil, err
	}
	return trim, nil
}

// lockCompanionFarEnds takes the row lock of every child at the far end of a
// link this write is about to drop.
//
// Without it the stranding check is a read two transactions can pass on each
// other's soon-to-be-deleted data: with links A-B and C-B, where B depends on
// them, a writer narrowing A's plan and one narrowing C's each still see the
// other edge, both conclude B stays covered, and both commit.
//
// Order: ascending by id, the order every companion writer uses. The subject's
// row is already locked above, so a far end BELOW it can only be acquired
// against that order — those go NOWAIT and surface as the retriable
// ErrCompanionLockBusy rather than blocking into a deadlock.
func (s *StudentService) lockCompanionFarEnds(
	ctx context.Context,
	stats *domain.OperationStats,
	studentID int64,
	farEnds []int64,
) error {
	ordered := make([]int64, 0, len(farEnds))
	for _, id := range farEnds {
		if id > 0 && id != studentID {
			ordered = append(ordered, id)
		}
	}
	if len(ordered) == 0 {
		return nil
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	for _, id := range ordered {
		lock := "UPDATE"
		if id < studentID {
			lock = "UPDATE NOWAIT"
		}
		_, found, lockStats, err := s.store.FindRecord(ctx, id, lock)
		stats.Add(lockStats)
		switch {
		case err == nil:
			// A deleted or foreign row is skipped, exactly as the stranding
			// check skips it.
			_ = found
		case errors.Is(err, domain.ErrStudentLockBusy):
			return domain.ErrCompanionLockBusy
		default:
			return err
		}
	}
	return nil
}

// decideStranding is the verdict itself, against the state the database holds
// right now. Links of excludeID are ignored as cover because the caller is
// about to delete them; pass 0 to count every stored link.
func (s *StudentService) decideStranding(
	ctx context.Context,
	stats *domain.OperationStats,
	excludeID int64,
	removed []int64,
	removedDays map[int64][]string,
) error {
	if len(removed) == 0 {
		return nil
	}
	if s.companions == nil {
		return errors.New("people directory application: companion capability is not bound")
	}
	covered, err := s.companions.DaysCoveredExcluding(ctx, removed, excludeID)
	if err != nil {
		return err
	}
	records, recordStats, err := s.store.ListRecordsByIDs(ctx, removed)
	stats.Add(recordStats)
	if err != nil {
		return err
	}
	byID := make(map[int64]domain.StudentRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}

	for _, id := range removed {
		record, ok := byID[id]
		if !ok {
			continue // deleted or another tenant — nothing left to strand
		}
		if domain.HasCompanionNote(record.DepartureCompanionNote) {
			continue // the free-text note carries the detail for every day
		}
		accompanied := domain.DeparturePlan{
			AllowedDepartureModes: record.AllowedDepartureModes,
			DepartureDays:         record.DepartureDays,
		}.AccompaniedDays()
		for _, day := range removedDays[id] {
			if !accompanied[day] {
				continue // their plan does not claim "Anderes Kind" on this day
			}
			if covered[id][day] {
				continue // another child still walks with them on this day
			}
			return domain.ErrCompanionWouldLoseDeparture
		}
	}
	return nil
}

// noteToStore answers the value the companion-note column must hold after this
// write: the supplied note while the resolved plan still allows an accompanied
// day, NULL otherwise. The free-text "mit wem" must never outlive the mode that
// justifies it, whichever projection drove the change.
func noteToStore(resolved domain.DeparturePlan, note *string) *string {
	if resolved.AllowedDepartureModes.HasMode(domain.DepartureAccompanied) {
		return note
	}
	return nil
}

// applyPlanToRecord reflects what was actually persisted back onto the row the
// caller gets, so it renders the stored plan rather than the one it submitted.
func applyPlanToRecord(record domain.StudentRecord, resolved domain.DeparturePlan, note *string) domain.StudentRecord {
	record.AllowedDepartureModes = resolved.AllowedDepartureModes
	record.DepartureDays = resolved.DepartureDays
	record.BusDays = resolved.BusDays
	record.PickupDays = resolved.PickupDays
	status := resolved.AllowedDepartureModes.LegacyPickupStatus()
	record.PickupStatus = &status
	record.DepartureCompanionNote = note
	return record
}
