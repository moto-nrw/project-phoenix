package enrollmenthttp_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// heldGuardianStudents parks in the guard after the student locks and before
// the profile lock, where a full guardian deletion must wait for the guard.
type heldGuardianStudents struct {
	owners.EnrollmentStudentMatches
	gate *renewalGate
}

func (s heldGuardianStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*peopledirectory.StudentState, error) {
	students, err := s.EnrollmentStudentMatches.FindByIDsForUpdate(ctx, ids)
	if err == nil {
		s.gate.park(ctx)
	}
	return students, err
}

func TestGuardianDeletionWaitsForAccountlessParentGuard(t *testing.T) {
	t.Parallel()
	rig := setupRenewalRig(t)
	student := enrolledChild(t, rig.db, "Lina", "Löschreihenfolge", existingChildBirthday)
	profile := testpkg.CreateTestGuardianProfileNamed(t, rig.db, "Eva", "Löschreihenfolge", "delete-guard-order")
	link := testpkg.CreateTestStudentGuardianLink(t, rig.db, student.ID, profile.ID, securityruntime.GuardianRolePrimaryGuardian)
	require.NotNil(t, profile.Email)
	services, _, err := api.NewTestPeopleServices(testpkg.NewDatabase(t, rig.db), slog.Default())
	require.NoError(t, err)
	gate := newRenewalGate()
	gate.armed.Store(true)
	t.Cleanup(gate.open)
	hold := owners.NewEnrollmentGuardianHold(testpkg.NewDatabase(t, rig.db), rig.repos.StudentGuardian,
		heldGuardianStudents{EnrollmentStudentMatches: rig.repos.Student, gate: gate}, rig.repos.GuardianProfile, nil)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	guarded := make(chan error, 1)
	go func() {
		guarded <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			grants, err := hold.HoldGuardianEmailStudentsPermission(txCtx, *profile.Email, []int64{student.ID}, nil,
				testpkg.Tenant(t), securityruntime.GuardianPermissionEnrollmentSubmit)
			if err == nil && !grants[student.ID] {
				return peopledirectory.ErrGuardianNotFound
			}
			return err
		})
	}()
	holder := awaitHolder(t, gate.reached, guarded, "the parent guard after its student lock")
	deleted := make(chan error, 1)
	go func() {
		deleted <- testpkg.TenantCommand3(t, ctx, rig.db, services.Guardian.DeleteGuardianWithLinks,
			profile.ID, []int64{link.ID}, rig.creatorID)
	}()
	awaitWaiter(t, rig.db, holder.pid, deleted, "the full deletion must wait for the held student")
	gate.open()
	require.NoError(t, awaitRenewal(t, guarded), "the guard must acquire the profile and commit without a deadlock")
	require.NoError(t, awaitRenewal(t, deleted), "the full deletion commits after the guard")
	gone, err := testpkg.TenantQuery1(t, ctx, rig.db, services.Guardian.GetGuardianByID, profile.ID)
	require.Error(t, err)
	require.Nil(t, gone)
}
