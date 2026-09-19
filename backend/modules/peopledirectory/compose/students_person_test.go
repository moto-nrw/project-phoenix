package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The data import resolves a matched person to its student row (#2708):
// alumni stay visible, foreign tenants and unknown persons do not.
func TestStudentDirectoryListsStudentsByPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, db, "Anna", "ByPerson", "1a")
	graduate := testpkg.CreateTestStudent(t, db, "Carl", "ByPerson", "4c")
	_, foreignTenant := otherTenantContext(t, db)
	foreign := testpkg.CreateTestStudentForTenant(t, db, foreignTenant, "Fremd", "ByPerson", "1a")
	_, err := module.GraduateStudents(ctx, []int64{graduate.ID})
	require.NoError(t, err)

	rows, err := module.ListStudentsByPersonID(ctx, []int64{first.PersonID, graduate.PersonID, foreign.PersonID, first.PersonID, 0, -1})
	require.NoError(t, err)
	require.Len(t, rows, 2, "the foreign tenant's student and unknown persons are not returned")
	assert.Equal(t, first.ID, rows[0].ID)
	assert.Equal(t, first.PersonID, rows[0].PersonID)
	assert.Equal(t, graduate.ID, rows[1].ID)
	assert.True(t, rows[1].IsAlumnus(), "alumni stay visible so a re-import can see the lifecycle status")

	empty, err := module.ListStudentsByPersonID(ctx, []int64{0})
	require.NoError(t, err)
	assert.Empty(t, empty)

	// The enrollment profile patch carries the directory columns the import
	// writes: group, the child's own address and the supervisor notes.
	street := "Kinderweg 3"
	notes := "kommt mit dem Rad"
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, first.ID, peopledirectory.EnrollmentProfilePatch{
		AddressSet: true, AddressStreet: &street, SupervisorNotesSet: true, SupervisorNotes: &notes,
	}))
	record, err := module.ReadEnrollmentStudent(ctx, first.ID, "")
	require.NoError(t, err)
	assert.Equal(t, &street, record.AddressStreet)
	assert.Nil(t, record.AddressCity)
	assert.Equal(t, &notes, record.SupervisorNotes)
}
