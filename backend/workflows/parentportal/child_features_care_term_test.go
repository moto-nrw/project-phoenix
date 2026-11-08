package parentportal_test

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// batchSettingsStub serves the feature settings as one batch, like the
// production settings binding: a key outside the requested batch fails, so a
// setting the batch forgets cannot pass unnoticed.
type batchSettingsStub struct {
	parentSettingsStub
}

func (s batchSettingsStub) ResolveManyForTenant(_ context.Context, _ int64, keys []string) (parentService.SettingsSnapshot, error) {
	return batchSnapshot{stub: s.parentSettingsStub, keys: keys}, nil
}

type batchSnapshot struct {
	stub parentSettingsStub
	keys []string
}

func (b batchSnapshot) Bool(key string) (bool, error) {
	if !slices.Contains(b.keys, key) {
		return false, fmt.Errorf("%s is not in the batch", key)
	}
	return b.stub.ResolveBoolForTenant(context.Background(), 0, key)
}

func (b batchSnapshot) String(key string) (string, error) {
	if !slices.Contains(b.keys, key) {
		return "", fmt.Errorf("%s is not in the batch", key)
	}
	return b.stub.ResolveStringForTenant(context.Background(), 0, key)
}

// The features of a child name its school's care term (#6082), from the same
// batch as the other feature settings, also once the child's care has ended.
func TestChildFeaturesNameTheSchoolsCareTerm(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         owners.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       repos.ParentChildReads(),
		StatusDayRepo:   repos.StudentStatusDay,
		StudentRepo:     owners.NewParentStudentReads(reposPeople.Student),
		CareExceptions:  owners.NewParentCareExceptions(repos.CarePlan()),
		Settings: batchSettingsStub{parentSettingsStub{
			stringValues: map[string]string{
				parentService.KeyGuardianParentInviteMode:   parentService.ParentInviteModeDisabled,
				parentService.KeyEnrollmentCareFacilityTerm: "GTS",
			},
		}},
		MealPlan: availableMealPlan(false),
		Logger:   slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})

	flags, err := svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, "GTS", flags.CareFacilityTerm)

	endCareFor(t, db, chain.StudentID)
	flags, err = svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.True(t, flags.CareEnded)
	assert.Equal(t, "GTS", flags.CareFacilityTerm)
}
