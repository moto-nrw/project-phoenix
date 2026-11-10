package importpkg

import (
	"context"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// Step 7 of #5552 (ADR 0067 addendum of 2026-11-08, exception E3 in its
// batch form; PR #6558): in a batch segment the profiles of the enrollment
// batch rows are created in one statement right after the persons, the
// membership of each row stays where the row path creates it, and the care
// profiles go in one statement after the rows. A row with a group or a
// permanent hint keeps the whole enrollment in place: the group's foreign
// key lock and the note insert follow the care insert in the owner's unit.

// preparedEnrollment is a child whose profile the batch created before the
// row loop.
type preparedEnrollment struct {
	created ports.CreatedEnrollmentStudent
	input   ports.EnrollmentStudent
	patch   ports.EnrollmentProfilePatch
	// statusSet: the status was taken before the profile statement, where
	// the row path takes the first child's (before the class gate).
	statusSet bool
}

// enrollmentBatchRow reports whether the row's child goes into the
// enrollment batch: no group and no permanent hint.
func enrollmentBatchRow(row importModels.StudentImportRow) bool {
	return row.GroupID == nil && strings.TrimSpace(row.SupervisorNotes) == ""
}

// createEnrollmentProfiles creates the profiles of the segment's enrollment
// batch rows in one statement and returns them by row. The status of the
// first of them is taken here, before the statement waits for the class
// gate, as the row path takes the first child's status before its profile
// insert waits there; every later row takes its status in the loop, after
// the gate, as on the row path (#5552 step 7, finding 1).
func (c *StudentImportConfig) createEnrollmentProfiles(ctx context.Context, rows []importModels.StudentImportRow, persons []ports.Person) ([]*preparedEnrollment, error) {
	prepared := make([]*preparedEnrollment, len(rows))
	if c.EnrollmentBatches == nil {
		return prepared, nil
	}
	var indices []int
	var inputs []ports.EnrollmentStudent
	for i, row := range rows {
		if !enrollmentBatchRow(row) {
			continue
		}
		input, patch, err := enrollmentInputFromRow(persons[i].ID, row)
		if err != nil {
			return nil, err
		}
		entry := &preparedEnrollment{input: input, patch: patch}
		if len(indices) == 0 {
			c.applyEnrollmentStatus(&entry.input, row)
			entry.statusSet = true
		}
		prepared[i] = entry
		indices = append(indices, i)
		inputs = append(inputs, entry.input)
	}
	if len(inputs) == 0 {
		return prepared, nil
	}
	created, err := c.EnrollmentBatches.CreateEnrollmentProfiles(ctx, inputs)
	if err != nil {
		return nil, err
	}
	for k, i := range indices {
		prepared[i].created = created[k]
	}
	return prepared, nil
}
