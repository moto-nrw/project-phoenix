package parentportal_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

func buildHolidayRegistrationFamilyService(t *testing.T) (*parentService.Portal, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:     owners.ParentGuardianChanges(),
		DeparturePolicy:     owners.ParentDeparturePolicy(),
		GuardianPolicy:      owners.NewParentGuardianPolicy(),
		Fingerprints:        owners.SecurityFingerprints{},
		Declarations:        owners.ParentDeclarationPolicy(),
		Locales:             owners.ParentPortalLocales(),
		Runtime:             owners.ParentPortalRuntime{},
		ChildRepo:           repos.ParentChildReads(),
		PersonRepo:          owners.NewParentPersonReads(people.Person),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile),
		GuardianPhoneRepo:   owners.NewParentGuardianPhoneReads(people.GuardianPhoneNumber),
		ChildDataAccess:     owners.NewParentChildDataAccess(owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).DataAccessLog),
		Logger:              slog.Default(),
		CarePlanPolicy:      owners.NewParentCarePlanPolicy(),
	})
	return svc, db
}

func prefillReads(t *testing.T, db *bun.DB, studentID int64) int {
	t.Helper()
	var reads int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log
		WHERE resource_type = 'holiday_prefill' AND student_id = ?`, studentID).Scan(context.Background(), &reads))
	return reads
}

// TestHolidayRegistrationFamilyOffersOnlyPermittedOwnChildren pins #4662: the
// prefill offers a child only where the account's relationship grants
// parent_portal.enrollment.submit at a school of the Träger, and records each
// read in the school's data access log. A child of another family, a child of
// the account without the permission and a child at another Träger's school
// are not offered and not read.
func TestHolidayRegistrationFamilyOffersOnlyPermittedOwnChildren(t *testing.T) {
	t.Parallel()
	svc, db := buildHolidayRegistrationFamilyService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := db.ExecContext(ctx, `UPDATE users.persons SET birthday = '2018-03-01' WHERE id = ?`, chain.PersonID)
	require.NoError(t, err)

	pickupOnly := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "2b")
	testpkg.CreateTestStudentGuardianLink(t, db, pickupOnly.ID, chain.GuardianProfileID, securityruntime.GuardianRolePickupOnly)
	foreign := testpkg.CreateTestStudent(t, db, "Tom", "Fremd", "1a")
	foreignGuardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Frieda", "Fremd", "frieda.fremd")
	testpkg.CreateTestStudentGuardianLink(t, db, foreign.ID, foreignGuardian.ID, securityruntime.GuardianRolePrimaryGuardian)

	family, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, 42, []int64{chain.TenantID})
	require.NoError(t, err)
	require.Len(t, family.Children, 1, "only the child whose relationship grants the registration")
	child := family.Children[0]
	assert.Equal(t, chain.StudentID, child.StudentID)
	assert.Equal(t, chain.TenantID, child.SchoolID)
	assert.Equal(t, "Felix", child.FirstName)
	assert.Equal(t, "Schneider", child.LastName)
	require.NotNil(t, child.BirthDate)
	assert.Equal(t, calendar.NewDate(2018, 3, 1), *child.BirthDate)
	require.NotNil(t, family.Guardian)
	assert.Equal(t, "Sabine", family.Guardian.FirstName)
	assert.Equal(t, chain.Email, family.Guardian.Email)

	assert.Equal(t, 1, prefillReads(t, db, chain.StudentID), "the read of the offered child is in the school's audit")
	assert.Zero(t, prefillReads(t, db, pickupOnly.ID), "a child without the permission is not read")
	assert.Zero(t, prefillReads(t, db, foreign.ID), "a child of another family is not read")

	other, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, 43, []int64{chain.TenantID + 1_000_000})
	require.NoError(t, err)
	assert.Empty(t, other.Children, "a child at a school of another Träger is not offered")
	assert.Nil(t, other.Guardian)
	assert.Equal(t, 1, prefillReads(t, db, chain.StudentID), "nothing is read for another Träger")
}
