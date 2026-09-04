package users_test

import (
	"fmt"
	"testing"

	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGuardiansQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupGuardianService(t, db)
	add := func(from, to int) {
		for i := from; i < to; i++ {
			testpkg.CreateTestGuardianProfile(t, db, fmt.Sprintf("guardian-budget-%d", i))
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueries(t, db)
	run := func() []string {
		counter.Reset()
		_, err := service.ListGuardians(testpkg.Ctx(t), nil)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.users.list_guardians.reads", large)
}

func TestStudentGuardiansQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupGuardianService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Guardian", "Budget", "1a")
	add := func(from, to int) {
		for i := from; i < to; i++ {
			profile := testpkg.CreateTestGuardianProfile(t, db, fmt.Sprintf("student-guardian-budget-%d", i))
			_, err := service.LinkGuardianToStudent(testpkg.Ctx(t), usersService.StudentGuardianCreateRequest{
				StudentID: student.ID, GuardianProfileID: profile.ID, RelationshipType: "guardian",
			})
			require.NoError(t, err)
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueries(t, db)
	run := func() []string {
		counter.Reset()
		_, err := service.GetStudentGuardians(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.users.student_guardians.reads", large)
}
