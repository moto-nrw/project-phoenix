package services

import (
	"context"
	"fmt"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	auditClassList "github.com/moto-nrw/project-phoenix/modules/auditlog/classlist"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The audited class-list administration (#2382, #3355) lives with School
// Membership, but two of its collaborators only exist once this factory has
// been built: the People Directory student lookup behind the duplicate guard
// and the "Zuordnen" hint, and the fail-closed Audit command behind the
// change trail. They are adapted to the owner's ports and bound here.

// classListEntryAdministrationBinder is what the School Membership module
// exposes for that late binding.
type classListEntryAdministrationBinder interface {
	BindClassListEntryAdministration(schoolmembership.ClassListEntryStudents, schoolmembership.ClassListEntryTrail)
}

// bindClassListEntryAdministration installs both ports. A capability without
// the binder cannot serve the Klassenliste screen at all, so the composition
// refuses to continue silently.
func bindClassListEntryAdministration(
	membership schoolmembership.Capability,
	persons peopledirectory.Capability,
	audit auditModels.Command,
) error {
	binder, ok := membership.(classListEntryAdministrationBinder)
	if !ok {
		return fmt.Errorf("class list entry administration: school membership %T cannot bind it", membership)
	}
	if persons == nil || audit == nil {
		return fmt.Errorf("class list entry administration: people directory and audit command are required")
	}
	binder.BindClassListEntryAdministration(classListEntryStudents{persons: persons}, classListEntryTrail{audit: audit})
	return nil
}

// classListEntryStudents answers "is this name already a child of that class"
// through the People Directory, matching like the legacy student lookup did:
// case-insensitive and trimmed on all three keys, alumni excluded — a
// graduate must not block a new child of the same name. It is the same
// two-step the class-list import already resolves its guard with.
//
// One difference to the legacy SQL, which joined users.persons directly: the
// directory leaves soft-deleted persons out, so a student whose person row is
// deleted no longer blocks an entry. That row is a broken state either way,
// and the owner is the only one allowed to decide which persons exist.
type classListEntryStudents struct{ persons peopledirectory.Capability }

func (d classListEntryStudents) ListStudentIDsByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]int64, error) {
	first, last := strings.TrimSpace(firstName), strings.TrimSpace(lastName)
	if first == "" || last == "" {
		return nil, nil
	}
	// Unpaged on purpose: absence is decided over every namesake, not over
	// the first page of a sorted listing.
	matches, err := d.persons.SearchPersons(ctx, peopledirectory.PersonFilter{FirstNameEquals: first, LastNameEquals: last})
	if err != nil {
		return nil, fmt.Errorf("class list entry student lookup: search persons: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(matches))
	for _, person := range matches {
		personIDs = append(personIDs, person.ID)
	}
	students, err := d.persons.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("class list entry student lookup: list students: %w", err)
	}
	class := strings.TrimSpace(schoolClass)
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() || !strings.EqualFold(strings.TrimSpace(student.SchoolClass), class) {
			continue
		}
		ids = append(ids, student.ID)
	}
	return ids, nil
}

func (d classListEntryStudents) IsEnrolledStudent(ctx context.Context, studentID int64) (bool, error) {
	students, err := d.persons.ListStudentsByID(ctx, []int64{studentID})
	if err != nil {
		return false, fmt.Errorf("class list entry student lookup: find student: %w", err)
	}
	for _, student := range students {
		if student.ID == studentID {
			return !student.IsAlumnus(), nil
		}
	}
	return false, nil
}

// classListEntryTrail appends the change through the single Audit command, so
// the row joins the producer's transaction and a refused append aborts it.
type classListEntryTrail struct{ audit auditModels.Command }

func (t classListEntryTrail) AppendClassListEntryChange(ctx context.Context, change schoolmembership.ClassListEntryChange) error {
	return t.audit.Append(ctx, &auditClassList.ClassListEntryChange{
		EntryID: change.EntryID, Action: change.Action, OldValue: change.OldValue,
		NewValue: change.NewValue, MatchedStudentID: change.MatchedStudentID, ChangedBy: change.ChangedBy,
	})
}

// classListEntryRosterReader adapts the owner capability to the narrow reader
// the enrollment class roster and the class day view consume: they append the
// entries of one class, or of every class, to the Klassenverband and need
// nothing else from the owner.
type classListEntryRosterReader struct {
	entries schoolmembership.ClassListEntries
}

// NewClassListEntryRosterReader binds the class roster's class-list reader to
// the School Membership capability that owns the entries.
func NewClassListEntryRosterReader(entries schoolmembership.ClassListEntries) enrollmentCompose.ClassListEntries {
	if entries == nil {
		panic("class list entry roster reader: the school membership capability is required")
	}
	return classListEntryRosterReader{entries: entries}
}

func (r classListEntryRosterReader) ListClassListEntries(ctx context.Context, schoolClass string) ([]enrollmentCompose.ClassListEntry, error) {
	values, err := r.entries.ListClassListEntriesInDisplayOrder(ctx, schoolmembership.ClassListEntryFilter{SchoolClass: schoolClass})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentCompose.ClassListEntry, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentCompose.ClassListEntry{
			ID: value.ID, FirstName: value.FirstName, LastName: value.LastName, SchoolClass: value.SchoolClass,
		})
	}
	return result, nil
}
