package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The class-list entries composed with their student directory and audit
// trail serve the audited writes without a later binding; a missing port is
// a construction error.
func TestNewClassListEntriesTakesItsPortsAtConstruction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	deps := Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment, ChildQuota: newChildQuotas()}
	students := &recordingStudents{byName: map[string][]int64{}, enrolled: map[int64]bool{}}
	trail := &recordingTrail{}

	_, err := NewClassListEntries(deps, nil, trail)
	require.ErrorContains(t, err, "need a student directory and an audit trail")
	_, err = NewClassListEntries(deps, students, nil)
	require.ErrorContains(t, err, "need a student directory and an audit trail")

	entries, err := NewClassListEntries(deps, students, trail)
	require.NoError(t, err)
	actor := testpkg.CreateTestAccount(t, db, "cle-construction-actor@test.local")
	created, err := entries.AddClassListEntry(testpkg.Ctx(t), schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Ida", LastName: "Konstrukt", SchoolClass: "3k"},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)
	require.Len(t, trail.changes, 1)
	assert.Equal(t, created.ID, trail.changes[0].EntryID)
}
