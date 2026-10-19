package studentintegration_test

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
	module := buildStudentOwnersModule(t, db)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, db, "Anna", "ByPerson", "1a")
	graduate := testpkg.CreateTestStudent(t, db, "Carl", "ByPerson", "4c")
	_, foreignTenant := otherTenantContext(t, db)
	foreign := testpkg.CreateTestStudentForTenant(t, db, foreignTenant, "Fremd", "ByPerson", "1a")
	_, err := buildStudentMembership(t, db).Graduate(ctx, []int64{graduate.ID})
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
	// writes: group and the child's own address. Its „Betreuernotizen" cell
	// becomes the child's carried-over permanent hint (#4268).
	street := "Kinderweg 3"
	notes := " kommt mit dem Rad "
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, first.ID, peopledirectory.EnrollmentProfilePatch{
		AddressSet: true, AddressStreet: &street, PermanentHint: &notes,
	}))
	record, err := module.ReadEnrollmentStudent(ctx, first.ID, "")
	require.NoError(t, err)
	assert.Equal(t, &street, record.AddressStreet)
	assert.Nil(t, record.AddressCity)
	var hint struct {
		Body       string `bun:"body"`
		Kind       string `bun:"kind"`
		Visibility string `bun:"visibility"`
	}
	require.NoError(t, db.NewRaw(`SELECT body, kind, visibility FROM users.student_notes
		WHERE student_id = ? AND origin = 'master_data' AND deleted_at IS NULL`, first.ID).Scan(ctx, &hint))
	assert.Equal(t, "kommt mit dem Rad", hint.Body)
	assert.Equal(t, "permanent", hint.Kind)
	assert.Equal(t, "all_staff", hint.Visibility)
}
