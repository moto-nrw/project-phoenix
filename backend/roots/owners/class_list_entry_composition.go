package owners

import (
	"context"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

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
