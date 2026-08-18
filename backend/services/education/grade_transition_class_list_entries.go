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
// Unlike the Klassenlehrer assignments there is deliberately NO ledger: an
// entry carries no access scope, so the revert reverses the mapping's renames
// directly (to→from). Entries a graduation deleted are NOT restored by a
// revert — they are name-only rows destined for deletion, and re-creating
// children's names the school already discarded would invert data
// minimization. The audit trail records every rewrite either way.

// remapClassListEntries follows the transition's class renames for
// users.class_list_entries. Affected rows are deleted and re-inserted (mirror
// of remapClassTeacherAssignments): simultaneous renames ("1a"→"2a" while
// "2a"→"3a") would otherwise collide with the unique name+class index
// mid-sequence. A re-insert that would duplicate a surviving entry or a
// student now holding the target class is skipped — the child already has
// exactly one row on the class list.
func (s *GradeTransitionService) remapClassListEntries(
	ctx context.Context,
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

	for _, entry := range affected {
		if err := s.classListEntryRepo.Delete(ctx, entry.ID); err != nil {
			return fmt.Errorf("failed to delete class list entry: %w", err)
		}
	}

	for _, entry := range affected {
		target := renames[strings.TrimSpace(entry.SchoolClass)]
		if err := s.reinsertClassListEntry(ctx, entry, target, kept, accountID); err != nil {
			return err
		}
	}
	return nil
}

// revertClassListEntries reverses the transition's renames (to→from) for the
// entries. Same delete/re-insert shape as the apply; graduated entries have
// no reverse mapping and stay deleted. An entry created in a rename-target
// class between apply and revert is renamed back with the rest — the revert
// cannot tell it apart without a ledger, and a wrongly-renamed name-only row
// is a visible, trivially fixable outcome.
func (s *GradeTransitionService) revertClassListEntries(
	ctx context.Context,
	mappings []*education.GradeTransitionMapping,
	accountID int64,
) error {
	if s.classListEntryRepo == nil {
		return nil
	}
	renames := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil || mapping.IsGraduating() {
			continue
		}
		renames[strings.TrimSpace(*mapping.ToClass)] = strings.TrimSpace(mapping.FromClass)
	}
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

	for _, entry := range affected {
		if err := s.classListEntryRepo.Delete(ctx, entry.ID); err != nil {
			return fmt.Errorf("failed to delete class list entry: %w", err)
		}
	}

	for _, entry := range affected {
		target := renames[strings.TrimSpace(entry.SchoolClass)]
		if err := s.reinsertClassListEntry(ctx, entry, target, kept, accountID); err != nil {
			return err
		}
	}
	return nil
}

// reinsertClassListEntry re-creates one remapped entry under its target class
// and writes the audit row. An empty target (graduation) or a duplicate at
// the target (surviving entry or a student of that name) records a deletion
// instead of inserting a second class-list row for the same child.
func (s *GradeTransitionService) reinsertClassListEntry(
	ctx context.Context,
	entry *userModels.ClassListEntry,
	target string,
	kept map[string]bool,
	accountID int64,
) error {
	oldValue := entry.DisplayValue()
	if target != "" && !kept[classListEntryKey(entry.FirstName, entry.LastName, target)] {
		duplicate, err := s.classListEntryDuplicatesStudent(ctx, entry, target)
		if err != nil {
			return err
		}
		if !duplicate {
			replacement := &userModels.ClassListEntry{
				FirstName:   entry.FirstName,
				LastName:    entry.LastName,
				SchoolClass: target,
				CreatedBy:   entry.CreatedBy,
			}
			if err := s.classListEntryRepo.Create(ctx, replacement); err != nil {
				return fmt.Errorf("failed to create class list entry: %w", err)
			}
			kept[classListEntryKey(entry.FirstName, entry.LastName, target)] = true
			return s.auditClassListEntryChange(ctx, replacement.ID, auditModels.ClassListEntryActionUpdated, oldValue, replacement.DisplayValue(), accountID)
		}
	}
	return s.auditClassListEntryChange(ctx, entry.ID, auditModels.ClassListEntryActionDeleted, oldValue, "", accountID)
}

// classListEntryDuplicatesStudent reports whether a regular student with the
// entry's name already holds the target class (post-rename state: the student
// writes of this transition ran first).
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
