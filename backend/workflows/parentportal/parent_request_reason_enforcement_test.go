package parentportal_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// reasonPolicySettings turns the absence approval gate on and serves one
// configured value of operations.parent_request_reason_policy.
type reasonPolicySettings struct {
	parentService.SettingsService
	policy string
}

func (reasonPolicySettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case parentService.KeyParentSickNoteEnabled,
		parentService.KeyParentSickReportsEnabled, parentService.KeyParentExcusedReportsEnabled,
		parentService.KeyParentSickRequiresApproval,
		parentService.KeyParentExcusedRequiresApproval:
		return true, nil
	default:
		return false, nil
	}
}

func (s reasonPolicySettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == parentService.KeyParentRequestReasonPolicy {
		return s.policy, nil
	}
	return "", nil
}

func buildReasonPolicyServices(t *testing.T, policy string) (*parentService.Portal, careplan.ExcusedAbsenceRequests, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	excused, err := api.NewTestExcusedAbsenceRequests(api.ExcusedRequestTestOptions{
		CarePlan: repos.CarePlan(), Logger: slog.Default(),
	}, api.ExcusedRequestTestPeople{Students: reposPeople.Student, Persons: reposPeople.Person, Events: reposPeople.ParentRequestEvent}, nil, nil)
	require.NoError(t, err)
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:    api.ParentGuardianChanges(),
		DeparturePolicy:    api.ParentDeparturePolicy(),
		GuardianPolicy:     api.NewParentGuardianPolicy(),
		Fingerprints:       api.SecurityFingerprints{},
		Declarations:       api.ParentDeclarationPolicy(),
		Locales:            api.ParentPortalLocales(),
		Runtime:            api.ParentPortalRuntime{},
		CarePlan:           api.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:             api.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:       careProfiles(t, db),
		ChildRepo:          repos.ParentChildReads(),
		StatusDayRepo:      repos.StudentStatusDay,
		StudentRepo:        api.NewParentStudentReads(reposPeople.Student),
		CareExceptions:     api.NewParentCareExceptions(repos.CarePlan()),
		Settings:           reasonPolicySettings{policy: policy},
		ExcusedRequests:    api.NewParentExcusedAbsenceRequests(excused),
		ExcusedRequestRepo: repos.ExcusedAbsenceRequest,
		Logger:             slog.Default(),

		DataRequests: api.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: api.NewParentCarePlanPolicy(),
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
		{parentService.ReasonPolicyNobody, false},
		{parentService.ReasonPolicyStaff, false},
		{parentService.ReasonPolicyGuardians, true},
		{parentService.ReasonPolicyBoth, true},
		{"", true}, // unknown value reads as the strictest policy
	}
	for _, tc := range cases {
		t.Run("policy="+tc.policy, func(t *testing.T) {
			t.Parallel()
			svc, _, db := buildReasonPolicyServices(t, tc.policy)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ctx := testpkg.WithPackageTenantRuntime(context.Background())

			res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
				[]calendar.Date{calendar.TodayDate().AddDays(3)}, "  ", absencerecords.StudentStatusDayExcused, nil)
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
		{parentService.ReasonPolicyNobody, false},
		{parentService.ReasonPolicyGuardians, false},
		{parentService.ReasonPolicyStaff, true},
		{parentService.ReasonPolicyBoth, true},
	}
	for _, tc := range cases {
		t.Run("policy="+tc.policy, func(t *testing.T) {
			t.Parallel()
			svc, requests, db := buildReasonPolicyServices(t, tc.policy)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ctx := testpkg.WithPackageTenantRuntime(context.Background())

			res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
				[]calendar.Date{calendar.TodayDate().AddDays(3)}, "Familienfeier", absencerecords.StudentStatusDayExcused, nil)
			require.NoError(t, err)

			decideErr := testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, err := requests.Decide(txCtx, careplan.ExcusedRequestDecideInput{
					RequestID:      res.PendingRequest.ID,
					Approve:        true,
					ReviewedBy:     chain.AccountID,
					ReasonRequired: parentService.RequestReasonPolicy(tc.policy).RequiresReason(true),
				})
				return err
			})
			if tc.wantRefusal {
				require.ErrorIs(t, decideErr, careplan.ErrParentRequestReasonRequired)
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

	svc, requests, db := buildReasonPolicyServices(t, parentService.ReasonPolicyNobody)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate().AddDays(3)}, "Familienfeier", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	decideErr := testpkg.WithTenantTx(t, adminCtx(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := requests.Decide(txCtx, careplan.ExcusedRequestDecideInput{
			RequestID: res.PendingRequest.ID, Approve: false, ReviewedBy: chain.AccountID,
			ReasonRequired: false,
		})
		return err
	})
	require.ErrorIs(t, decideErr, careplan.ErrExcusedRequestRejectReasonRequired)
}
