package peopledirectory

import (
	"context"
	"time"
)

// AnonymizeRetiredStaffPerson replaces the identifiers of a retired staff
// member's person with the same placeholders as a deleted child's and
// tombstones the row while its updated_at still matches the read value
// (#3963). The automatic deletion of staff calls it once no live staff
// membership names the person; records of other people keep pointing at
// the anonymized row. False means a concurrent edit moved the row.
func (m *Module) AnonymizeRetiredStaffPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	if personID <= 0 {
		return false, invalid("person ID is required")
	}
	if updatedAt.IsZero() {
		return false, invalid("read person revision is required")
	}
	return m.engine.AnonymizeDeletedStudentPerson(ctx, personID, updatedAt)
}
