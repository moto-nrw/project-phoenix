package parentportal_test

import (
	"context"
	"log/slog"
	"testing"

	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// The parent portal must know before the family types anything whether the
// note is required (#2267). The flag follows the school's policy: "guardians"
// and "both" ask the family, "staff" and "nobody" do not.
func TestChildFeaturesReasonRequiredFollowsPolicy(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	for _, tc := range []struct {
		policy string
		want   bool
	}{
		{parentportal.ReasonPolicyNobody, false},
		{parentportal.ReasonPolicyStaff, false},
		{parentportal.ReasonPolicyGuardians, true},
		{parentportal.ReasonPolicyBoth, true},
	} {
		svc := parentportalcompose.New(parentportalcompose.Dependencies{
			GuardianChanges: api.ParentGuardianChanges(),
			DeparturePolicy: api.ParentDeparturePolicy(),
			GuardianPolicy:  api.NewParentGuardianPolicy(),
			Fingerprints:    api.SecurityFingerprints{},
			Declarations:    api.ParentDeclarationPolicy(),
			Locales:         api.ParentPortalLocales(),
			Runtime:         api.ParentPortalRuntime{},
			CarePlan:        api.NewParentCarePlanCommands(repos.CarePlan(), nil),
			People:          api.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
			CareProfiles:    careProfiles(t, db),
			ChildRepo:       repos.ParentChildReads(),
			StatusDayRepo:   repos.StudentStatusDay,
			StudentRepo:     api.NewParentStudentReads(reposPeople.Student),
			CareExceptions:  api.NewParentCareExceptions(repos.CarePlan()),
			Settings: parentSettingsStub{
				stringValues: map[string]string{
					parentportal.KeyGuardianParentInviteMode:  parentportal.ParentInviteModeDisabled,
					parentportal.KeyParentRequestReasonPolicy: tc.policy,
				},
			},
			MealPlan: availableMealPlan(false),
			Logger:   slog.Default(),

			DataRequests: api.NewParentDataRequestCommands(repos.CarePlan()),

			CarePlanPolicy: api.NewParentCarePlanPolicy(),
		})

		flags, err := svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
		require.NoError(t, err, "policy %q", tc.policy)
		assert.Equal(t, tc.want, flags.ReasonRequired, "policy %q", tc.policy)
	}
}
