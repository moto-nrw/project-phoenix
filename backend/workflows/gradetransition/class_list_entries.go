package gradetransition

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// Audit actions of the class-list rewrite, as stored by the Audit owner.
const (
	ClassListAuditCreated = "created"
	ClassListAuditUpdated = "updated"
	ClassListAuditDeleted = "deleted"
)

// The class-list-only entries follow the rollover the same way the students
// do: a promoted class carries its entries along, a graduating class loses
// them. Every rewrite lands in the transition's class-list ledger and the
// audit trail; the revert replays the ledger instead of reversing the
// mappings, so entries the apply never touched and entries created between
// apply and revert stay untouched.

// classListRenameLookup resolves an entry's class against the renames: exact
// (trimmed) match first, then a normalized fallback, because an entry's
// cohort identity is the NORMALIZED class everywhere else. A normalized key
// two mappings collapse onto with different targets is ambiguous and
// excluded from the fallback.
type classListRenameLookup struct {
	exact      map[string]string
	normalized map[string]string
}

func newClassListRenameLookup(renames map[string]string) classListRenameLookup {
	normalized := make(map[string]string, len(renames))
	ambiguous := make(map[string]bool)
	for from, to := range renames {
		key := schoolclass.Normalize(from)
		if existing, seen := normalized[key]; seen && existing != to {
			ambiguous[key] = true
			continue
		}
		normalized[key] = to
	}
	for key := range ambiguous {
		delete(normalized, key)
	}
	return classListRenameLookup{exact: renames, normalized: normalized}
}

// target returns the rename target ("" graduates) and whether the class is
// part of the transition at all.
func (l classListRenameLookup) target(schoolClass string) (string, bool) {
	if to, ok := l.exact[strings.TrimSpace(schoolClass)]; ok {
		return to, true
	}
	to, ok := l.normalized[schoolclass.Normalize(schoolClass)]
	return to, ok
}

// classListEntryKey is the duplicate identity of an entry: normalized name
// and class, mirroring the unique index.
func classListEntryKey(firstName, lastName, schoolClass string) string {
	return strings.ToLower(strings.TrimSpace(firstName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(lastName)) + "\x00" +
		schoolclass.Normalize(schoolClass)
}

func classListDisplayValue(firstName, lastName, schoolClass string) string {
	return strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName) + " (" + strings.TrimSpace(schoolClass) + ")"
}

// classListStudentNames caches, per class, the names of the enrolled
// children a class-list entry must not duplicate.
type classListStudentNames struct {
	workflow *Workflow
	byClass  map[string]map[string]bool
}

func (w *Workflow) newClassListStudentNames() *classListStudentNames {
	return &classListStudentNames{workflow: w, byClass: make(map[string]map[string]bool)}
}

// duplicatesStudent reports whether a regular student with the entry's name
// already holds the class (post-rename state on apply, post-restore state on
// revert: the student writes of the transition run first either way).
func (c *classListStudentNames) duplicatesStudent(ctx context.Context, firstName, lastName, schoolClass string) (bool, error) {
	names, cached := c.byClass[schoolClass]
	if !cached {
		students, err := c.workflow.deps.Directory.ListStudentsByClasses(ctx, []string{schoolClass})
		if err != nil {
			return false, fmt.Errorf("failed to check class list entry against students: %w", err)
		}
		names = make(map[string]bool, len(students))
		if len(students) > 0 {
			ids := make([]int64, 0, len(students))
			for _, student := range students {
				ids = append(ids, student.ID)
			}
			resolved, err := c.workflow.deps.Directory.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return false, fmt.Errorf("failed to check class list entry against students: %w", err)
			}
			for _, name := range resolved {
				names[classListEntryKey(name.FirstName, name.LastName, schoolClass)] = true
			}
		}
		c.byClass[schoolClass] = names
	}
	return names[classListEntryKey(firstName, lastName, schoolClass)], nil
}

func (c *classListStudentNames) invalidate(schoolClass string) {
	delete(c.byClass, schoolClass)
}

// remapClassListEntries follows the transition's class renames for the
// class-list entries. Affected rows are deleted and re-inserted (mirror of
// the Klassenlehrer remap); a re-insert that would duplicate a surviving
// entry or a student now holding the target class is skipped.
func (w *Workflow) remapClassListEntries(ctx context.Context, actor Actor, transitionID int64, mappings []schoolstructure.TransitionMapping) error {
	renames := classRenames(mappings)
	if len(renames) == 0 {
		return nil
	}
	lookup := newClassListRenameLookup(renames)
	entries, err := w.deps.Membership.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	if err != nil {
		return fmt.Errorf("failed to list class list entries: %w", err)
	}
	var affected []schoolmembership.ClassListEntry
	kept := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if _, hit := lookup.target(entry.SchoolClass); hit {
			affected = append(affected, entry)
			continue
		}
		kept[classListEntryKey(entry.FirstName, entry.LastName, entry.SchoolClass)] = true
	}
	if len(affected) == 0 {
		return nil
	}
	names := w.newClassListStudentNames()
	var ledger []schoolstructure.TransitionClassListEntry
	for _, entry := range affected {
		if err := w.deps.Membership.DeleteClassListEntry(ctx, entry.ID); err != nil {
			return fmt.Errorf("failed to delete class list entry: %w", err)
		}
		ledger = append(ledger, schoolstructure.TransitionClassListEntry{
			TransitionID: transitionID, FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: entry.SchoolClass,
			Action: schoolstructure.LedgerActionRemoved,
		})
	}
	for _, entry := range affected {
		target, _ := lookup.target(entry.SchoolClass)
		created, err := w.reinsertClassListEntry(ctx, actor, entry, target, kept, names)
		if err != nil {
			return err
		}
		if created != nil {
			createdID := created.ID
			ledger = append(ledger, schoolstructure.TransitionClassListEntry{
				TransitionID: transitionID, EntryID: &createdID, FirstName: created.FirstName, LastName: created.LastName,
				SchoolClass: created.SchoolClass, Action: schoolstructure.LedgerActionCreated,
			})
		}
	}
	if err := w.deps.Structure.AppendTransitionClassListLedger(ctx, ledger); err != nil {
		return fmt.Errorf("failed to record class list entry ledger: %w", translateStructureError(err))
	}
	return nil
}

// reinsertClassListEntry re-creates one remapped entry under its target
// class and writes the audit row, returning the created row (nil when
// nothing was inserted). An empty target (graduation) or a duplicate at the
// target records a deletion instead.
func (w *Workflow) reinsertClassListEntry(ctx context.Context, actor Actor, entry schoolmembership.ClassListEntry, target string, kept map[string]bool, names *classListStudentNames) (*schoolmembership.ClassListEntry, error) {
	oldValue := classListDisplayValue(entry.FirstName, entry.LastName, entry.SchoolClass)
	if target != "" && !kept[classListEntryKey(entry.FirstName, entry.LastName, target)] {
		duplicate, err := names.duplicatesStudent(ctx, entry.FirstName, entry.LastName, target)
		if err != nil {
			return nil, err
		}
		if !duplicate {
			replacement, err := w.deps.Membership.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
				ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: target},
				CreatedBy:            entry.CreatedBy,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create class list entry: %w", err)
			}
			kept[classListEntryKey(entry.FirstName, entry.LastName, target)] = true
			return &replacement, w.auditClassListEntry(ctx, actor, replacement.ID, ClassListAuditUpdated, oldValue,
				classListDisplayValue(replacement.FirstName, replacement.LastName, replacement.SchoolClass))
		}
	}
	return nil, w.auditClassListEntry(ctx, actor, entry.ID, ClassListAuditDeleted, oldValue, "")
}

// revertClassListEntries replays the ledger backwards: rows the apply
// created are deleted again, rows it removed (promotions AND graduations)
// are restored with their original display form. Entries the admin changed
// since the apply win: a created row the admin already deleted or edited is
// left alone AND its paired removed entry is not restored; a restore
// colliding with a surviving entry or with a student of that name and class
// is skipped.
func (w *Workflow) revertClassListEntries(ctx context.Context, actor Actor, transitionID int64, mappings []schoolstructure.TransitionMapping) error {
	ledger, err := w.deps.Structure.ListTransitionClassListLedger(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("failed to load class list entry ledger: %w", translateStructureError(err))
	}
	if len(ledger) == 0 {
		return nil
	}
	entries, err := w.deps.Membership.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	if err != nil {
		return fmt.Errorf("failed to list class list entries: %w", err)
	}
	current := make(map[string]schoolmembership.ClassListEntry, len(entries))
	byID := make(map[int64]schoolmembership.ClassListEntry, len(entries))
	for _, entry := range entries {
		current[classListEntryKey(entry.FirstName, entry.LastName, entry.SchoolClass)] = entry
		byID[entry.ID] = entry
	}
	ledgerCreated := make(map[string]bool)
	deletedCreated := make(map[string]bool)
	for _, item := range ledger {
		if item.Action != schoolstructure.LedgerActionCreated {
			continue
		}
		key := classListEntryKey(item.FirstName, item.LastName, item.SchoolClass)
		ledgerCreated[key] = true
		row, found := resolveLedgerCreatedRow(item, current, byID)
		if !found {
			continue
		}
		if err := w.deps.Membership.DeleteClassListEntry(ctx, row.ID); err != nil {
			return fmt.Errorf("failed to delete class list entry: %w", err)
		}
		delete(current, classListEntryKey(row.FirstName, row.LastName, row.SchoolClass))
		deletedCreated[key] = true
		if err := w.auditClassListEntry(ctx, actor, row.ID, ClassListAuditDeleted, classListDisplayValue(row.FirstName, row.LastName, row.SchoolClass), ""); err != nil {
			return err
		}
	}
	lookup := newClassListRenameLookup(classRenames(mappings))
	names := w.newClassListStudentNames()
	for _, item := range ledger {
		if item.Action != schoolstructure.LedgerActionRemoved {
			continue
		}
		if target, hit := lookup.target(item.SchoolClass); hit && target != "" {
			pairKey := classListEntryKey(item.FirstName, item.LastName, target)
			if ledgerCreated[pairKey] && !deletedCreated[pairKey] {
				continue // the admin removed the renamed child after the apply
			}
		}
		key := classListEntryKey(item.FirstName, item.LastName, item.SchoolClass)
		if _, taken := current[key]; taken {
			continue
		}
		duplicate, err := names.duplicatesStudent(ctx, item.FirstName, item.LastName, item.SchoolClass)
		if err != nil {
			return err
		}
		if duplicate {
			continue // the reactivated student of that name IS the row now
		}
		create := schoolmembership.CreateClassListEntry{
			ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: item.FirstName, LastName: item.LastName, SchoolClass: item.SchoolClass},
		}
		if actor.AccountID > 0 {
			accountID := actor.AccountID
			create.CreatedBy = &accountID
		}
		restored, err := w.deps.Membership.CreateClassListEntry(ctx, create)
		if err != nil {
			return fmt.Errorf("failed to restore class list entry: %w", err)
		}
		current[key] = restored
		names.invalidate(item.SchoolClass)
		if err := w.auditClassListEntry(ctx, actor, restored.ID, ClassListAuditCreated, "", classListDisplayValue(restored.FirstName, restored.LastName, restored.SchoolClass)); err != nil {
			return err
		}
	}
	return nil
}

// resolveLedgerCreatedRow returns the row a created ledger item inserted, or
// false when the revert must leave it alone: the admin deleted it, or edited
// it since the apply. The lookup goes through the recorded row ID, because
// an edit that keeps the normalized identity is invisible to an identity
// lookup; ledger rows written before the ID was recorded fall back to it.
func resolveLedgerCreatedRow(item schoolstructure.TransitionClassListEntry, byKey map[string]schoolmembership.ClassListEntry, byID map[int64]schoolmembership.ClassListEntry) (schoolmembership.ClassListEntry, bool) {
	if item.EntryID == nil {
		row, found := byKey[classListEntryKey(item.FirstName, item.LastName, item.SchoolClass)]
		return row, found
	}
	row, found := byID[*item.EntryID]
	if !found {
		return schoolmembership.ClassListEntry{}, false
	}
	if row.FirstName != item.FirstName || row.LastName != item.LastName || row.SchoolClass != item.SchoolClass {
		return schoolmembership.ClassListEntry{}, false
	}
	return row, true
}

func (w *Workflow) auditClassListEntry(ctx context.Context, actor Actor, entryID int64, action, oldValue, newValue string) error {
	if err := w.deps.AppendClassListEntryAudit(ctx, actor, ClassListEntryAudit{EntryID: entryID, Action: action, OldValue: oldValue, NewValue: newValue}); err != nil {
		return fmt.Errorf("failed to record class list entry change: %w", err)
	}
	return nil
}
