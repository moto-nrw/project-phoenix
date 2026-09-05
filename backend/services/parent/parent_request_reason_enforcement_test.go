package parent_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// reasonPolicySettings turns the absence approval gate on and serves one
// configured value of operations.parent_request_reason_policy.
type reasonPolicySettings struct {
	configService.SettingsService
	policy string
}

func (reasonPolicySettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case configModels.KeyParentSickNoteEnabled,
		configModels.KeyParentSickRequiresApproval,
		configModels.KeyParentExcusedRequiresApproval:
		return true, nil
	default:
		return false, nil
	}
}

func (s reasonPolicySettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == configModels.KeyParentRequestReasonPolicy {
		return s.policy, nil
	}
	return "", nil
}

func buildReasonPolicyServices(t *testing.T, policy string) (parentService.Service, absenceSvc.ExcusedAbsenceRequestService, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	excused := absenceSvc.NewExcusedAbsenceRequestServiceWithPolicy(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		nil, nil, nil,
		testpkg.AbsenceRequestReviewPolicy{},
		usersSvc.NewParentRequestEventRecorder(repos.ParentRequestEvent),
		slog.Default(),
		db,
	)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		Settings:            reasonPolicySettings{policy: policy},
		ExcusedRequests:     excused,
		ExcusedRequestRepo:  repos.ExcusedAbsenceRequest,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, excused, db
}

// TestGuardianNoteFollowsReasonPolicy pins story 28 on the guardian side: a
// school that asks nobody for a reason accepts a submission without a note,
// and one that asks the family refuses it.
func TestGuardianNoteFollowsReasonPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		policy      string
		wantRefusal bool
	}{
		{configModels.ReasonPolicyNobody, false},
		{configModels.ReasonPolicyStaff, false},
		{configModels.ReasonPolicyGuardians, true},
		{configModels.ReasonPolicyBoth, true},
		{"", true}, // unknown value reads as the strictest policy
	}
	for _, tc := range cases {
		t.Run("policy="+tc.policy, func(t *testing.T) {
			t.Parallel()
			svc, _, db := buildReasonPolicyServices(t, tc.policy)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ctx := testpkg.WithPackageTenantRuntime(context.Background())

			res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
				[]timezone.Date{timezone.TodayDate().AddDays(3)}, "  ", activeModels.StudentStatusDayExcused, nil)
			if tc.wantRefusal {
				require.ErrorIs(t, err, parentService.ErrEmptyNote)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, res.PendingRequest)
			assert.Empty(t, res.PendingRequest.Note)
		})
	}
}

// TestStaffApprovalReasonFollowsReasonPolicy pins story 28 on the staff side:
// an approval needs a reason only under staff|both, a rejection always does.
func TestStaffApprovalReasonFollowsReasonPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		policy      string
		wantRefusal bool
	}{
		{configModels.ReasonPolicyNobody, false},
		{configModels.ReasonPolicyGuardians, false},
		{configModels.ReasonPolicyStaff, true},
		{configModels.ReasonPolicyBoth, true},
	}
	for _, tc := range cases {
		t.Run("policy="+tc.policy, func(t *testing.T) {
			t.Parallel()
			svc, requests, db := buildReasonPolicyServices(t, tc.policy)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ctx := testpkg.WithPackageTenantRuntime(context.Background())

			res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
				[]timezone.Date{timezone.TodayDate().AddDays(3)}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
			require.NoError(t, err)

			decideErr := testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, err := requests.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
					RequestID:      res.PendingRequest.ID,
					Approve:        true,
					ReviewedBy:     chain.AccountID,
					ReasonRequired: usersSvc.ReasonRequiredFor(tc.policy, true),
				})
				return err
			})
			if tc.wantRefusal {
				require.ErrorIs(t, decideErr, usersSvc.ErrParentRequestReasonRequired)
				return
			}
			require.NoError(t, decideErr)
		})
	}
}

// TestStaffRejectionAlwaysNeedsAReason pins that the policy never makes a
// rejection reasonless: a refusal the family cannot understand generates the
// phone call the whole feature exists to avoid.
func TestStaffRejectionAlwaysNeedsAReason(t *testing.T) {
	t.Parallel()

	svc, requests, db := buildReasonPolicyServices(t, configModels.ReasonPolicyNobody)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate().AddDays(3)}, "Familienfeier", activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	decideErr := testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := requests.Decide(txCtx, absenceSvc.ExcusedRequestDecideInput{
			RequestID: res.PendingRequest.ID, Approve: false, ReviewedBy: chain.AccountID,
			ReasonRequired: false,
		})
		return err
	})
	require.ErrorIs(t, decideErr, absenceSvc.ErrExcusedRequestRejectReasonRequired)
}
