package parentportal_test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// membershipLocks is the Identity & Access capability the root binds as the
// school membership lock of every parent write (#6529).
type membershipLocks interface {
	LockActiveSchoolMembership(ctx context.Context, accountID int64) (bool, error)
}

// parentMemberships, guardStudentGuardians, guardProfiles and guardStudents
// bind the real ports the child write guard locks through, as the root does.
func parentMemberships(t testing.TB, db *bun.DB) membershipLocks {
	t.Helper()
	return owners.NewPeopleIdentityAccess(testpkg.NewDatabase(t, db), nil)
}

func guardStudentGuardians(t testing.TB, db *bun.DB) parentService.StudentGuardianReads {
	t.Helper()
	return owners.NewParentStudentGuardianReads(api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).StudentGuardian)
}

func guardProfiles(t testing.TB, db *bun.DB) parentService.GuardianProfiles {
	t.Helper()
	return owners.NewParentGuardianProfileReads(api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianProfile)
}

func guardStudents(t testing.TB, db *bun.DB) parentService.StudentReads {
	t.Helper()
	return owners.NewParentStudentReads(api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student)
}
