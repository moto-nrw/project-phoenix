package education

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The class-list-only entries (#2382) follow the school-year rollover the
// same way the students do: a promoted class carries its entries along
// (1a→2a), a graduating class loses them. Left untouched, a "1a" entry would
// silently point at the NEXT incoming 1a cohort.
//
// Every rewrite lands in the transition's class-list-entry ledger
// (education.grade_transition_class_list_entries, mirror of the Klassenlehrer
// ledger): the revert replays exactly what the apply recorded instead of
// reversing the mappings — a reverse rename cannot distinguish an entry the
// apply renamed into a class from a pre-existing entry of that class it never
// touched, and it would also drag entries created between apply and revert
// along. Graduated entries are restored by the revert like the graduated
// students themselves are reactivated. The audit trail records every rewrite
// either way.

// remapClassListEntries follows the transition's class renames for
// users.class_list_entries. Affected rows are deleted and re-inserted (mirror
// of remapClassTeacherAssignments): simultaneous renames ("1a"→"2a" while
// "2a"→"3a") would otherwise collide with the unique name+class index
// mid-sequence. A re-insert that would duplicate a surviving entry or a
// student now holding the target class is skipped — the child already has
// exactly one row on the class list. Every delete and insert is recorded in
// the transition's class-list-entry ledger so the revert can replay it.
func (s *GradeTransitionService) remapClassListEntries(
	ctx context.Context,
	transitionID int64,
	mappings []*education.GradeTransitionMapping,
	accountID int64,
) error {
	if s.classListEntryRepo == nil {
		return nil
	}
	renames := classTeacherRenames(mappings)
	if len(renames) == 0 {
		return nil
	}

	entries, err := s.classListEntryRepo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list class list entries: %w", err)
	}

	var affected []*userModels.ClassListEntry
	kept := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if _, hit := renames[strings.TrimSpace(entry.SchoolClass)]; hit {
			affected = append(affected, entry)
			continue
		}
		kept[classListEntryKey(entry.FirstName, entry.LastName, entry.SchoolClass)] = true
	}
	if len(affected) == 0 {
		return nil
	}

	var ledger []*education.GradeTransitionClassListEntry

	for _, entry := range affected {
		if err := s.classListEntryRepo.Delete(ctx, entry.ID); err != nil {
			return fmt.Errorf("failed to delete class list entry: %w", err)
		}
		ledger = append(ledger, &education.GradeTransitionClassListEntry{
			TransitionID: transitionID,
			FirstName:    entry.FirstName,
			LastName:     entry.LastName,
			SchoolClass:  entry.SchoolClass,
			Action:       education.ClassTeacherActionRemoved,
		})
	}

	for _, entry := range affected {
		target := renames[strings.TrimSpace(entry.SchoolClass)]
		created, err := s.reinsertClassListEntry(ctx, entry, target, kept, accountID)
		if err != nil {
			return err
		}
		if created != nil {
			ledger = append(ledger, &education.GradeTransitionClassListEntry{
				TransitionID: transitionID,
				FirstName:    created.FirstName,
				LastName:     created.LastName,
				SchoolClass:  created.SchoolClass,
				Action:       education.ClassTeacherActionCreated,
			})
		}
	}

	if err := s.transitionRepo.CreateClassListEntryHistoryBatch(ctx, ledger); err != nil {
		return fmt.Errorf("failed to record class list entry ledger: %w", err)
	}
	return nil
}

// revertClassListEntries replays the transition's class-list-entry ledger
// backwards: rows the apply created are deleted again, rows it removed
// (promotions AND graduations) are restored with their original display form.
// Entries the apply never touched — a pre-existing entry of a rename target,
// or one created between apply and revert — have no ledger entry and stay
// untouched. Entries the admin changed since the apply win: a created row the
// admin already deleted is not deleted again AND its paired removed entry is
// not restored (the admin deliberately took the child off the list), and a
// restore colliding with a surviving entry or a student of that name and
// class is skipped — the child already has exactly one row on the list.
func (s *GradeTransitionService) revertClassListEntries(
	ctx context.Context,
	transitionID int64,
	mappings []*education.GradeTransitionMapping,
	accountID int64,
) error {
	if s.classListEntryRepo == nil {
		return nil
	}

	ledger, err := s.transitionRepo.GetClassListEntryHistory(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("failed to load class list entry ledger: %w", err)
	}
	if len(ledger) == 0 {
		return nil
	}

	entries, err := s.classListEntryRepo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list class list entries: %w", err)
	}
	current := make(map[string]*userModels.ClassListEntry, len(entries))
	for _, entry := range entries {
		current[classListEntryKey(entry.FirstName, entry.LastName, entry.SchoolClass)] = entry
	}

	// ledgerCreated: identities the apply inserted at all. deletedCreated:
	// the subset this revert actually found and deleted — an identity in the
	// first set but not the second was removed by the admin after the apply,
	// and its paired restore must not resurrect the child.
	ledgerCreated := make(map[string]bool)
	deletedCreated := make(map[string]bool)
	for _, item := range ledger {
		if item.Action != education.ClassTeacherActionCreated {
			continue
		}
		key := classListEntryKey(item.FirstName, item.LastName, item.SchoolClass)
		ledgerCreated[key] = true
		row, ok := current[key]
		if !ok {
			continue
		}
		if err := s.classListEntryRepo.Delete(ctx, row.ID); err != nil {
			return fmt.Errorf("failed to delete class list entry: %w", err)
		}
		delete(current, key)
		deletedCreated[key] = true
		if err := s.auditClassListEntryChange(ctx, row.ID, auditModels.ClassListEntryActionDeleted, row.DisplayValue(), "", accountID); err != nil {
			return err
		}
	}

	renames := classTeacherRenames(mappings)

	for _, item := range ledger {
		if item.Action != education.ClassTeacherActionRemoved {
			continue
		}
		if target := renames[strings.TrimSpace(item.SchoolClass)]; target != "" {
			pairKey := classListEntryKey(item.FirstName, item.LastName, target)
			if ledgerCreated[pairKey] && !deletedCreated[pairKey] {
				continue // the admin removed the renamed child after the apply
			}
		}
		key := classListEntryKey(item.FirstName, item.LastName, item.SchoolClass)
		if _, taken := current[key]; taken {
			continue
		}
		probe := &userModels.ClassListEntry{FirstName: item.FirstName, LastName: item.LastName}
		duplicate, err := s.classListEntryDuplicatesStudent(ctx, probe, item.SchoolClass)
		if err != nil {
			return err
		}
		if duplicate {
			continue // the reactivated student of that name IS the row now
		}
		restored := &userModels.ClassListEntry{
			FirstName:   item.FirstName,
			LastName:    item.LastName,
			SchoolClass: item.SchoolClass,
		}
		if accountID > 0 {
			restored.CreatedBy = &accountID
		}
		if err := s.classListEntryRepo.Create(ctx, restored); err != nil {
			return fmt.Errorf("failed to restore class list entry: %w", err)
		}
		current[key] = restored
		if err := s.auditClassListEntryChange(ctx, restored.ID, auditModels.ClassListEntryActionCreated, "", restored.DisplayValue(), accountID); err != nil {
			return err
		}
	}
	return nil
}

// reinsertClassListEntry re-creates one remapped entry under its target class
// and writes the audit row, returning the created row (nil when nothing was
// inserted). An empty target (graduation) or a duplicate at the target
// (surviving entry or a student of that name) records a deletion instead of
// inserting a second class-list row for the same child.
func (s *GradeTransitionService) reinsertClassListEntry(
	ctx context.Context,
	entry *userModels.ClassListEntry,
	target string,
	kept map[string]bool,
	accountID int64,
) (*userModels.ClassListEntry, error) {
	oldValue := entry.DisplayValue()
	if target != "" && !kept[classListEntryKey(entry.FirstName, entry.LastName, target)] {
		duplicate, err := s.classListEntryDuplicatesStudent(ctx, entry, target)
		if err != nil {
			return nil, err
		}
		if !duplicate {
			replacement := &userModels.ClassListEntry{
				FirstName:   entry.FirstName,
				LastName:    entry.LastName,
				SchoolClass: target,
				CreatedBy:   entry.CreatedBy,
			}
			if err := s.classListEntryRepo.Create(ctx, replacement); err != nil {
				return nil, fmt.Errorf("failed to create class list entry: %w", err)
			}
			kept[classListEntryKey(entry.FirstName, entry.LastName, target)] = true
			return replacement, s.auditClassListEntryChange(ctx, replacement.ID, auditModels.ClassListEntryActionUpdated, oldValue, replacement.DisplayValue(), accountID)
		}
	}
	return nil, s.auditClassListEntryChange(ctx, entry.ID, auditModels.ClassListEntryActionDeleted, oldValue, "", accountID)
}

// classListEntryDuplicatesStudent reports whether a regular student with the
// entry's name already holds the target class (post-rename state on apply,
// post-restore state on revert: the student writes of the transition run
// first either way).
func (s *GradeTransitionService) classListEntryDuplicatesStudent(ctx context.Context, entry *userModels.ClassListEntry, target string) (bool, error) {
	if s.studentRepo == nil {
		return false, nil
	}
	students, err := s.studentRepo.FindByNameAndClass(ctx, entry.FirstName, entry.LastName, target)
	if err != nil {
		return false, fmt.Errorf("failed to check class list entry against students: %w", err)
	}
	return len(students) > 0, nil
}

func (s *GradeTransitionService) auditClassListEntryChange(ctx context.Context, entryID int64, action, oldValue, newValue string, accountID int64) error {
	if s.classListEntryAudit == nil {
		return nil
	}
	if err := s.classListEntryAudit.Create(ctx, &auditModels.ClassListEntryChange{
		EntryID:   entryID,
		Action:    action,
		OldValue:  oldValue,
		NewValue:  newValue,
		ChangedBy: accountID,
	}); err != nil {
		return fmt.Errorf("failed to record class list entry change: %w", err)
	}
	return nil
}

// classListEntryKey is the duplicate identity of an entry: normalized name
// and class, mirroring the unique index.
func classListEntryKey(firstName, lastName, schoolClass string) string {
	return strings.ToLower(strings.TrimSpace(firstName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(lastName)) + "\x00" +
		schoolclass.Normalize(schoolClass)
}
