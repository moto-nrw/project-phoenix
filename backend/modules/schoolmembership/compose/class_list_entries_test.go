package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The module composed with the class-list student directory and audit trail
// serves the audited writes without a later binding; only one of the two
// ports is a construction error.
func TestNewTakesTheClassListPortsAtConstruction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	deps := Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment, ChildQuota: newChildQuotas()}
	students := &recordingStudents{byName: map[string][]int64{}, enrolled: map[int64]bool{}}
	trail := &recordingTrail{}

	onlyTrail := deps
	onlyTrail.ClassListTrail = trail
	_, err := New(onlyTrail)
	require.ErrorContains(t, err, "need a student directory and an audit trail")
	onlyStudents := deps
	onlyStudents.ClassListStudents = students
	_, err = New(onlyStudents)
	require.ErrorContains(t, err, "need a student directory and an audit trail")

	deps.ClassListStudents, deps.ClassListTrail = students, trail
	entries, err := New(deps)
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
