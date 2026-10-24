package contracttest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/uptrace/bun"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
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
		_, err := testpkg.MeasuredTenantQuery1(t, testpkg.Ctx(t), db, counter, service.ListGuardians, nil)
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
			_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, service.LinkGuardianToStudent, peoplePorts.StudentGuardianCreateRequest{
				StudentID: student.ID, GuardianProfileID: profile.ID, RelationshipType: "guardian",
			})
			require.NoError(t, err)
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueries(t, db)
	run := func() []string {
		var statements []string
		err := testpkg.WithinTenantTransaction(testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			// Measure owner reads inside the request unit of work, not its setup.
			counter.Reset()
			_, err := testpkg.TenantQuery1(t, txCtx, db, service.GetStudentGuardians, student.ID)
			statements = counter.Operation("SELECT")
			return err
		})
		require.NoError(t, err)
		return statements
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.users.student_guardians.reads", large)
}
