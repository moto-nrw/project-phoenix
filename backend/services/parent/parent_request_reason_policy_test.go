package parent_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The parent portal must know before the family types anything whether the
// note is required (#2267). The flag follows the school's policy: "guardians"
// and "both" ask the family, "staff" and "nobody" do not.
func TestChildFeaturesReasonRequiredFollowsPolicy(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	for _, tc := range []struct {
		policy string
		want   bool
	}{
		{configModels.ReasonPolicyNobody, false},
		{configModels.ReasonPolicyStaff, false},
		{configModels.ReasonPolicyGuardians, true},
		{configModels.ReasonPolicyBoth, true},
	} {
		svc := parentService.NewService(parentService.ServiceConfig{
			ChildRepo:           repos.ParentChild,
			StatusDayRepo:       repos.StudentStatusDay,
			StudentRepo:         repos.Student,
			PickupExceptionRepo: repos.StudentPickupException,
			Settings: parentSettingsStub{
				stringValues: map[string]string{
					configModels.KeyGuardianParentInviteMode:  configModels.ParentInviteModeDisabled,
					configModels.KeyParentRequestReasonPolicy: tc.policy,
				},
			},
			MealPlan: availableMealPlan(false),
			DB:       db,
			Logger:   slog.Default(),
		})

		flags, err := svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
		require.NoError(t, err, "policy %q", tc.policy)
		assert.Equal(t, tc.want, flags.ReasonRequired, "policy %q", tc.policy)
	}
}
