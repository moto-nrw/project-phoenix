package application

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
)

// --- class list entries ---

func (s *Service) FindClassListEntry(ctx context.Context, id int64, lock string) (result domain.ClassListEntry, err error) {
	run := s.runRead
	if lock != "" {
		run = s.runWrite
	}
	err = run(ctx, "find_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindClassListEntry(txCtx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			result = domain.ClassListEntry{}
			return domain.ErrClassListEntryNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListClassListEntries(ctx context.Context, filter domain.ClassListEntryFilter) (result []domain.ClassListEntry, err error) {
	err = s.runRead(ctx, "list_class_list_entries", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClassListEntries(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateClassListEntry(ctx context.Context, fields domain.ClassListEntryFields, createdBy *int64) (result domain.ClassListEntry, err error) {
	err = s.runWrite(ctx, "create_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClassListEntry(txCtx, fields, createdBy)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateClassListEntry(ctx context.Context, id int64, fields domain.ClassListEntryFields) (result domain.ClassListEntry, err error) {
	err = s.runWrite(ctx, "update_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindClassListEntry(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrClassListEntryNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateClassListEntry(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteClassListEntry(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteClassListEntry(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- audited class-list administration (#2382) ---

// ErrClassListEntryAdministrationUnbound reports a class-list administration
// call on a module the composition root never handed the student directory
// and the audit trail. It is a wiring error, not a request outcome.
var ErrClassListEntryAdministrationUnbound = errors.New("school membership: class list entry administration is not bound")

// classListAdministration is the slot the two late-bound collaborators live
// in. A slot rather than two plain fields: the composition ratchet counts a
// dependency-typed field written from a setter as composition surface, which
// is shrink-only, and the pointer publishes both collaborators in one step so
// no request can observe half a binding.
type classListAdministration struct {
	value atomic.Pointer[classListCollaborators]
}

type classListCollaborators struct {
	students ports.StudentDirectory
	trail    ports.AuditTrail
}

func (s *classListAdministration) store(collaborators classListCollaborators) {
	s.value.Store(&collaborators)
}

func (s *classListAdministration) load() (classListCollaborators, error) {
	collaborators := s.value.Load()
	if collaborators == nil {
		return classListCollaborators{}, ErrClassListEntryAdministrationUnbound
	}
	return *collaborators, nil
}

// BindClassListEntryAdministration installs the two capabilities the audited
// flows need and that only exist once the composition root has built them.
// It is called once at composition time; a second call replaces both so test
// graphs can rebuild them.
func (s *Service) BindClassListEntryAdministration(students ports.StudentDirectory, trail ports.AuditTrail) {
	if students == nil || trail == nil {
		panic("school membership application: class list entry administration needs a student directory and an audit trail")
	}
	s.classListAdmin.store(classListCollaborators{students: students, trail: trail})
}

// ListClassListEntriesInDisplayOrder lists the entries the way a class list
// reads them: class (grade-aware), then last and first name with German
// collation. The plain listing's order is name-only and case-folded.
func (s *Service) ListClassListEntriesInDisplayOrder(ctx context.Context, filter domain.ClassListEntryFilter) (result []domain.ClassListEntry, err error) {
	err = s.runRead(ctx, "list_class_list_entries_in_display_order", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClassListEntriesInDisplayOrder(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// MatchingStudentIDs names the still-enrolled students sharing an entry's
// name and class: the hint for a deliberate "Zuordnen" resolution, never an
// automatic merge (#2382: gleichnamige Kinder dürfen nicht verwechselt
// werden).
func (s *Service) MatchingStudentIDs(ctx context.Context, fields domain.ClassListEntryFields) (result []int64, err error) {
	collaborators, err := s.classListAdmin.load()
	if err != nil {
		return nil, err
	}
	err = s.runRead(ctx, "match_class_list_entry_students", func(txCtx context.Context, _ *domain.OperationStats) error {
		result, err = collaborators.students.ListStudentIDsByNameAndClass(txCtx, fields.FirstName, fields.LastName, fields.SchoolClass)
		return err
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []int64{}
	}
	return result, nil
}

// AddClassListEntry creates one entry with the two duplicate guards and the
// audit row of the administration screen. The entry check is advisory — the
// unique index below is the race-safe backstop — while the student check has
// no constraint behind it at all: a student created concurrently surfaces as
// a match hint in the listing instead.
func (s *Service) AddClassListEntry(ctx context.Context, fields domain.ClassListEntryFields, changedBy int64) (result domain.ClassListEntry, err error) {
	collaborators, err := s.classListAdmin.load()
	if err != nil {
		return domain.ClassListEntry{}, err
	}
	err = s.runWrite(ctx, "add_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireClassListEntryNameFree(txCtx, collaborators, fields, 0, stats); err != nil {
			return err
		}
		var createdBy *int64
		if changedBy > 0 {
			createdBy = &changedBy
		}
		var createStats domain.OperationStats
		created, createStats, err := s.store.CreateClassListEntry(txCtx, fields, createdBy)
		stats.Add(createStats)
		if err != nil {
			return err
		}
		result = created
		return collaborators.trail.AppendClassListEntryChange(txCtx, domain.ClassListEntryChange{
			EntryID: created.ID, Action: domain.ClassListEntryActionCreated,
			NewValue: domain.ClassListEntryDisplayValue(created.Fields()), ChangedBy: changedBy,
		})
	})
	if err != nil {
		return domain.ClassListEntry{}, err
	}
	return result, nil
}

// ReviseClassListEntry renames an entry or moves it to another class. A
// revision that changes nothing is a no-op: it writes no row and leaves no
// audit trace, exactly like the screen's "save" on an untouched form.
func (s *Service) ReviseClassListEntry(ctx context.Context, id int64, fields domain.ClassListEntryFields, changedBy int64) (result domain.ClassListEntry, err error) {
	collaborators, err := s.classListAdmin.load()
	if err != nil {
		return domain.ClassListEntry{}, err
	}
	err = s.runWrite(ctx, "revise_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		current, err := s.lockClassListEntry(txCtx, id, stats)
		if err != nil {
			return err
		}
		oldValue := domain.ClassListEntryDisplayValue(current.Fields())
		newValue := domain.ClassListEntryDisplayValue(fields)
		if newValue == oldValue {
			result = current
			return nil
		}
		if err := s.requireClassListEntryNameFree(txCtx, collaborators, fields, id, stats); err != nil {
			return err
		}
		var updateStats domain.OperationStats
		updated, updateStats, err := s.store.UpdateClassListEntry(txCtx, id, fields)
		stats.Add(updateStats)
		if err != nil {
			return err
		}
		result = updated
		return collaborators.trail.AppendClassListEntryChange(txCtx, domain.ClassListEntryChange{
			EntryID: id, Action: domain.ClassListEntryActionUpdated,
			OldValue: oldValue, NewValue: domain.ClassListEntryDisplayValue(updated.Fields()), ChangedBy: changedBy,
		})
	})
	if err != nil {
		return domain.ClassListEntry{}, err
	}
	return result, nil
}

// RemoveClassListEntry deletes an entry and records what was removed.
func (s *Service) RemoveClassListEntry(ctx context.Context, id int64, changedBy int64) error {
	collaborators, err := s.classListAdmin.load()
	if err != nil {
		return err
	}
	return s.runWrite(ctx, "remove_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		current, err := s.lockClassListEntry(txCtx, id, stats)
		if err != nil {
			return err
		}
		deleteStats, err := s.store.DeleteClassListEntry(txCtx, id)
		stats.Add(deleteStats)
		if err != nil {
			return err
		}
		return collaborators.trail.AppendClassListEntryChange(txCtx, domain.ClassListEntryChange{
			EntryID: id, Action: domain.ClassListEntryActionDeleted,
			OldValue: domain.ClassListEntryDisplayValue(current.Fields()), ChangedBy: changedBy,
		})
	})
}

// ResolveClassListEntry resolves the entry as a duplicate of an existing
// student: the entry is deleted and the audit row records which student it
// was attached to, so the child keeps exactly one row on the class list. The
// target must BE the child the entry names — same name in the same class,
// matched like the "Mögliche Dublette" hint. Anything else would resolve the
// entry into an unrelated child and silently drop the named child.
func (s *Service) ResolveClassListEntry(ctx context.Context, id, studentID, changedBy int64) error {
	collaborators, err := s.classListAdmin.load()
	if err != nil {
		return err
	}
	return s.runWrite(ctx, "resolve_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		current, err := s.lockClassListEntry(txCtx, id, stats)
		if err != nil {
			return err
		}
		enrolled, err := collaborators.students.IsEnrolledStudent(txCtx, studentID)
		if err != nil {
			return err
		}
		if !enrolled {
			return domain.ErrClassListEntryStudentNotFound
		}
		matches, err := collaborators.students.ListStudentIDsByNameAndClass(txCtx,
			current.FirstName, current.LastName, current.SchoolClass)
		if err != nil {
			return err
		}
		if !containsID(matches, studentID) {
			return domain.ErrClassListEntryAssignMismatch
		}
		deleteStats, err := s.store.DeleteClassListEntry(txCtx, id)
		stats.Add(deleteStats)
		if err != nil {
			return err
		}
		matched := studentID
		return collaborators.trail.AppendClassListEntryChange(txCtx, domain.ClassListEntryChange{
			EntryID: id, Action: domain.ClassListEntryActionAssigned,
			OldValue:         domain.ClassListEntryDisplayValue(current.Fields()),
			MatchedStudentID: &matched, ChangedBy: changedBy,
		})
	})
}

func (s *Service) lockClassListEntry(ctx context.Context, id int64, stats *domain.OperationStats) (domain.ClassListEntry, error) {
	entry, found, queryStats, err := s.store.FindClassListEntry(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return domain.ClassListEntry{}, err
	}
	if !found {
		return domain.ClassListEntry{}, domain.ErrClassListEntryNotFound
	}
	return entry, nil
}

// requireClassListEntryNameFree rejects the name and class when another
// entry or any still-enrolled student already carries it. ignoreEntryID
// skips the entry being revised.
func (s *Service) requireClassListEntryNameFree(ctx context.Context, collaborators classListCollaborators, fields domain.ClassListEntryFields, ignoreEntryID int64, stats *domain.OperationStats) error {
	existing, listStats, err := s.store.ListClassListEntries(ctx, domain.ClassListEntryFilter{
		FirstName: fields.FirstName, LastName: fields.LastName, SchoolClass: fields.SchoolClass,
	})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	for _, entry := range existing {
		if entry.ID != ignoreEntryID {
			return domain.ErrClassListEntryDuplicate
		}
	}
	students, err := collaborators.students.ListStudentIDsByNameAndClass(ctx, fields.FirstName, fields.LastName, fields.SchoolClass)
	if err != nil {
		return err
	}
	if len(students) > 0 {
		return domain.ErrClassListEntryStudentExists
	}
	return nil
}

func containsID(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
