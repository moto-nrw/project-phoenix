package application

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// Class-list entry audit actions (#2382). Part of the audit wire format.
const (
	ClassListEntryActionCreated  = "created"
	ClassListEntryActionUpdated  = "updated"
	ClassListEntryActionDeleted  = "deleted"
	ClassListEntryActionAssigned = "assigned"
)

// ClassListEntryDisplayValue renders the audit-trail representation of an
// entry: "Vorname Nachname (Klasse)".
func ClassListEntryDisplayValue(fields schoolmembership.ClassListEntryFields) string {
	return strings.TrimSpace(fields.FirstName) + " " + strings.TrimSpace(fields.LastName) +
		" (" + strings.TrimSpace(fields.SchoolClass) + ")"
}

// classListEntryFieldsOf returns the writable part of an entry.
func classListEntryFieldsOf(entry schoolmembership.ClassListEntry) schoolmembership.ClassListEntryFields {
	return schoolmembership.ClassListEntryFields{FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: entry.SchoolClass}
}

// --- class list entries ---

func (s *Service) FindClassListEntry(ctx context.Context, id int64) (schoolmembership.ClassListEntry, error) {
	if id <= 0 {
		return schoolmembership.ClassListEntry{}, invalid("class list entry ID is required")
	}
	return s.findClassListEntry(ctx, id, "")
}

// FindClassListEntryForMutation locks the row for the caller's transaction.
func (s *Service) FindClassListEntryForMutation(ctx context.Context, id int64) (schoolmembership.ClassListEntry, error) {
	if id <= 0 {
		return schoolmembership.ClassListEntry{}, invalid("class list entry ID is required")
	}
	return s.findClassListEntry(ctx, id, "UPDATE")
}

func (s *Service) findClassListEntry(ctx context.Context, id int64, lock string) (result schoolmembership.ClassListEntry, err error) {
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
			result = schoolmembership.ClassListEntry{}
			return schoolmembership.ErrClassListEntryNotFound
		}
		return err
	})
	return result, err
}

// ListClassListEntries returns the entries matching the filter, ordered by
// last name, first name (both case-folded) and ID.
func (s *Service) ListClassListEntries(ctx context.Context, filter schoolmembership.ClassListEntryFilter) (result []schoolmembership.ClassListEntry, err error) {
	filter = normalizeClassListEntryFilter(filter)
	err = s.runRead(ctx, "list_class_list_entries", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClassListEntries(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateClassListEntry(ctx context.Context, input schoolmembership.CreateClassListEntry) (result schoolmembership.ClassListEntry, err error) {
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	if input.CreatedBy != nil && *input.CreatedBy <= 0 {
		input.CreatedBy = nil
	}
	fields, createdBy := input.ClassListEntryFields, input.CreatedBy
	err = s.runWrite(ctx, "create_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClassListEntry(txCtx, fields, createdBy)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateClassListEntry(ctx context.Context, input schoolmembership.UpdateClassListEntry) (result schoolmembership.ClassListEntry, err error) {
	if input.ID <= 0 {
		return schoolmembership.ClassListEntry{}, invalid("class list entry ID is required")
	}
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	id, fields := input.ID, input.ClassListEntryFields
	err = s.runWrite(ctx, "update_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindClassListEntry(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return schoolmembership.ErrClassListEntryNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateClassListEntry(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

// DeleteClassListEntry removes the entry; entries carry no tombstone.
func (s *Service) DeleteClassListEntry(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("class list entry ID is required")
	}
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

// ClassListAdministration are the two collaborators the audited flows need:
// the student directory behind the duplicate guard and the match hint, and
// the audit trail every change appends to. A service composed without them
// refuses the audited flows with ErrClassListEntryAdministrationUnbound.
type ClassListAdministration struct {
	Students schoolmembership.ClassListEntryStudents
	Trail    schoolmembership.ClassListEntryTrail
}

func (s *Service) classListAdministration() (ClassListAdministration, error) {
	if s.classList == nil {
		return ClassListAdministration{}, ErrClassListEntryAdministrationUnbound
	}
	return *s.classList, nil
}

// ListClassListEntriesInDisplayOrder lists the entries the way a class list
// reads them: class (grade-aware, so "2a" precedes "10a"), then last and
// first name in German collation. Every surface that renders entries next
// to students uses it, so the two orders cannot drift apart. The plain
// listing's order is name-only and case-folded.
func (s *Service) ListClassListEntriesInDisplayOrder(ctx context.Context, filter schoolmembership.ClassListEntryFilter) (result []schoolmembership.ClassListEntry, err error) {
	filter = normalizeClassListEntryFilter(filter)
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
// It is a read over whatever the rows carry, so it trims but does not
// validate: an entry with an empty field would otherwise fail the whole
// listing instead of simply matching nobody.
func (s *Service) MatchingStudentIDs(ctx context.Context, fields schoolmembership.ClassListEntryFields) (result []int64, err error) {
	fields.FirstName = strings.TrimSpace(fields.FirstName)
	fields.LastName = strings.TrimSpace(fields.LastName)
	fields.SchoolClass = strings.TrimSpace(fields.SchoolClass)
	collaborators, err := s.classListAdministration()
	if err != nil {
		return nil, err
	}
	err = s.runRead(ctx, "match_class_list_entry_students", func(txCtx context.Context, _ *domain.OperationStats) error {
		result, err = collaborators.Students.ListStudentIDsByNameAndClass(txCtx, fields.FirstName, fields.LastName, fields.SchoolClass)
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
func (s *Service) AddClassListEntry(ctx context.Context, input schoolmembership.AddClassListEntry) (result schoolmembership.ClassListEntry, err error) {
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	fields, changedBy := input.ClassListEntryFields, input.ChangedBy
	collaborators, err := s.classListAdministration()
	if err != nil {
		return schoolmembership.ClassListEntry{}, err
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
		return collaborators.Trail.AppendClassListEntryChange(txCtx, schoolmembership.ClassListEntryChange{
			EntryID: created.ID, Action: ClassListEntryActionCreated,
			NewValue: ClassListEntryDisplayValue(classListEntryFieldsOf(created)), ChangedBy: changedBy,
		})
	})
	if err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	return result, nil
}

// ReviseClassListEntry renames an entry or moves it to another class. A
// revision that changes nothing is a no-op: it writes no row and leaves no
// audit trace, exactly like the screen's "save" on an untouched form.
func (s *Service) ReviseClassListEntry(ctx context.Context, input schoolmembership.ReviseClassListEntry) (result schoolmembership.ClassListEntry, err error) {
	if input.ID <= 0 {
		return schoolmembership.ClassListEntry{}, invalid("class list entry ID is required")
	}
	if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	id, fields, changedBy := input.ID, input.ClassListEntryFields, input.ChangedBy
	collaborators, err := s.classListAdministration()
	if err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	err = s.runWrite(ctx, "revise_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		current, err := s.lockClassListEntry(txCtx, id, stats)
		if err != nil {
			return err
		}
		oldValue := ClassListEntryDisplayValue(classListEntryFieldsOf(current))
		newValue := ClassListEntryDisplayValue(fields)
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
		return collaborators.Trail.AppendClassListEntryChange(txCtx, schoolmembership.ClassListEntryChange{
			EntryID: id, Action: ClassListEntryActionUpdated,
			OldValue: oldValue, NewValue: ClassListEntryDisplayValue(classListEntryFieldsOf(updated)), ChangedBy: changedBy,
		})
	})
	if err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	return result, nil
}

// RemoveClassListEntry deletes an entry and records what was removed.
func (s *Service) RemoveClassListEntry(ctx context.Context, input schoolmembership.RemoveClassListEntry) error {
	if input.ID <= 0 {
		return invalid("class list entry ID is required")
	}
	id, changedBy := input.ID, input.ChangedBy
	collaborators, err := s.classListAdministration()
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
		return collaborators.Trail.AppendClassListEntryChange(txCtx, schoolmembership.ClassListEntryChange{
			EntryID: id, Action: ClassListEntryActionDeleted,
			OldValue: ClassListEntryDisplayValue(classListEntryFieldsOf(current)), ChangedBy: changedBy,
		})
	})
}

// ResolveClassListEntry resolves the entry as a duplicate of an existing
// student: the entry is deleted and the audit row records which student it
// was attached to, so the child keeps exactly one row on the class list. The
// target must BE the child the entry names — same name in the same class,
// matched like the "Mögliche Dublette" hint. Anything else would resolve the
// entry into an unrelated child and silently drop the named child.
func (s *Service) ResolveClassListEntry(ctx context.Context, input schoolmembership.ResolveClassListEntry) error {
	if input.ID <= 0 {
		return invalid("class list entry ID is required")
	}
	if input.StudentID <= 0 {
		return invalid("student ID is required")
	}
	id, studentID, changedBy := input.ID, input.StudentID, input.ChangedBy
	collaborators, err := s.classListAdministration()
	if err != nil {
		return err
	}
	return s.runWrite(ctx, "resolve_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		current, err := s.lockClassListEntry(txCtx, id, stats)
		if err != nil {
			return err
		}
		enrolled, err := collaborators.Students.IsEnrolledStudent(txCtx, studentID)
		if err != nil {
			return err
		}
		if !enrolled {
			return schoolmembership.ErrClassListEntryStudentNotFound
		}
		matches, err := collaborators.Students.ListStudentIDsByNameAndClass(txCtx,
			current.FirstName, current.LastName, current.SchoolClass)
		if err != nil {
			return err
		}
		if !containsID(matches, studentID) {
			return schoolmembership.ErrClassListEntryAssignMismatch
		}
		deleteStats, err := s.store.DeleteClassListEntry(txCtx, id)
		stats.Add(deleteStats)
		if err != nil {
			return err
		}
		matched := studentID
		return collaborators.Trail.AppendClassListEntryChange(txCtx, schoolmembership.ClassListEntryChange{
			EntryID: id, Action: ClassListEntryActionAssigned,
			OldValue:         ClassListEntryDisplayValue(classListEntryFieldsOf(current)),
			MatchedStudentID: &matched, ChangedBy: changedBy,
		})
	})
}

func (s *Service) lockClassListEntry(ctx context.Context, id int64, stats *domain.OperationStats) (schoolmembership.ClassListEntry, error) {
	entry, found, queryStats, err := s.store.FindClassListEntry(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return schoolmembership.ClassListEntry{}, err
	}
	if !found {
		return schoolmembership.ClassListEntry{}, schoolmembership.ErrClassListEntryNotFound
	}
	return entry, nil
}

// requireClassListEntryNameFree rejects the name and class when another
// entry or any still-enrolled student already carries it. ignoreEntryID
// skips the entry being revised.
func (s *Service) requireClassListEntryNameFree(ctx context.Context, collaborators ClassListAdministration, fields schoolmembership.ClassListEntryFields, ignoreEntryID int64, stats *domain.OperationStats) error {
	existing, listStats, err := s.store.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{
		FirstName: fields.FirstName, LastName: fields.LastName, SchoolClass: fields.SchoolClass,
	})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	for _, entry := range existing {
		if entry.ID != ignoreEntryID {
			return schoolmembership.ErrClassListEntryDuplicate
		}
	}
	students, err := collaborators.Students.ListStudentIDsByNameAndClass(ctx, fields.FirstName, fields.LastName, fields.SchoolClass)
	if err != nil {
		return err
	}
	if len(students) > 0 {
		return schoolmembership.ErrClassListEntryStudentExists
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
