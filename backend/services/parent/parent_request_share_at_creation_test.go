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
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildSharingAtCreationServices wires the parent service with everything the
// sharing path needs, so the recipient choice that travels with a creation is
// really written (and really refused) against the database.
func buildSharingAtCreationServices(t *testing.T) (parentService.Service, parentService.RequestSharingService, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
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
		ChildRepo:              repos.ParentChild,
		StatusDayRepo:          repos.StudentStatusDay,
		StudentRepo:            repos.Student,
		PersonRepo:             repos.Person,
		GuardianProfileRepo:    repos.GuardianProfile,
		StudentGuardianRepo:    repos.StudentGuardian,
		FamilyProtectionEvents: repos.FamilyProtection,
		ParentRequestShares:    repos.ParentRequestShare,
		ParentRequestEvents:    usersSvc.NewParentRequestEventRecorder(repos.ParentRequestEvent),
		ExcusedRequests:        excused,
		ExcusedRequestRepo:     repos.ExcusedAbsenceRequest,
		Settings: excusedApprovalSettings{
			sickRequiresApproval:    true,
			excusedRequiresApproval: true,
		},
		DB:     db,
		Logger: slog.Default(),
	})
	sharing, ok := svc.(parentService.RequestSharingService)
	require.True(t, ok)
	return svc, sharing, db
}

// TestShareTravelsWithTheCreation is finding 4: the recipient choice is stored
// in the same transaction as the request, so a family never ends up with a
// request none of the co-guardians they picked can see.
func TestShareTravelsWithTheCreation(t *testing.T) {
	t.Parallel()

	svc, sharing, db := buildSharingAtCreationServices(t)
	family := testpkg.CreateTestParentGuardianChain(t, db)
	co := testpkg.CreateTestCoGuardianForStudent(t, db, family.StudentID, "Klaus", "Zweitelternteil")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	res, err := svc.SubmitSickNote(ctx, family.AccountID, family.StudentID,
		[]timezone.Date{timezone.TodayDate().AddDays(3)}, "Familienfeier",
		activeModels.StudentStatusDayExcused, []int64{co.GuardianProfileID})
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest)

	state, err := sharing.GetRequestSharing(ctx, family.AccountID, family.StudentID,
		parentService.RequestShareExcused, res.PendingRequest.ID)
	require.NoError(t, err)
	var shared bool
	for _, recipient := range state.Recipients {
		if recipient.GuardianProfileID == co.GuardianProfileID {
			shared = recipient.Selected
		}
	}
	assert.True(t, shared, "the co-guardian picked at creation must be a recipient without a second call")
}

// TestForgedRecipientRollsTheRequestBack pins the atomicity: an id that is not
// a co-guardian of this child is refused, and because the share runs inside the
// creation's transaction no request row survives.
func TestForgedRecipientRollsTheRequestBack(t *testing.T) {
	t.Parallel()

	svc, _, db := buildSharingAtCreationServices(t)
	family := testpkg.CreateTestParentGuardianChain(t, db)
	stranger := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := svc.SubmitSickNote(ctx, family.AccountID, family.StudentID,
		[]timezone.Date{timezone.TodayDate().AddDays(4)}, "Familienfeier",
		activeModels.StudentStatusDayExcused, []int64{stranger.GuardianProfileID})
	require.ErrorIs(t, err, parentService.ErrRequestSharingInvalid)

	open, err := svc.ListExcusedRequests(ctx, family.AccountID, family.StudentID)
	require.NoError(t, err)
	assert.Empty(t, open, "a refused share must take the request row down with it")
}

// TestEmptyRecipientsWriteNoShareEvent pins that "shared with nobody" is the
// default state, not something the ledger records.
func TestEmptyRecipientsWriteNoShareEvent(t *testing.T) {
	t.Parallel()

	svc, sharing, db := buildSharingAtCreationServices(t)
	family := testpkg.CreateTestParentGuardianChain(t, db)
	co := testpkg.CreateTestCoGuardianForStudent(t, db, family.StudentID, "Klaus", "Zweitelternteil")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	res, err := svc.SubmitSickNote(ctx, family.AccountID, family.StudentID,
		[]timezone.Date{timezone.TodayDate().AddDays(5)}, "Familienfeier",
		activeModels.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.NotNil(t, res.PendingRequest)

	state, err := sharing.GetRequestSharing(ctx, family.AccountID, family.StudentID,
		parentService.RequestShareExcused, res.PendingRequest.ID)
	require.NoError(t, err)
	for _, recipient := range state.Recipients {
		if recipient.GuardianProfileID == co.GuardianProfileID {
			assert.False(t, recipient.Selected, "no recipients means no share")
		}
	}
}
