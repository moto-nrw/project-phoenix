package parentportal_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// buildSharingAtCreationServices wires the parent service with everything the
// sharing path needs, so the recipient choice that travels with a creation is
// really written (and really refused) against the database.
func buildSharingAtCreationServices(t *testing.T) (*parentService.Portal, parentService.RequestSharingService, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposCare := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	excused, err := api.NewTestExcusedAbsenceRequests(api.ExcusedRequestTestOptions{
		CarePlan: reposCare.CarePlan(), Logger: slog.Default(),
	}, api.ExcusedRequestTestPeople{Students: repos.Student, Persons: repos.Person, Events: repos.ParentRequestEvent}, nil, nil)
	require.NoError(t, err)
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:        api.ParentGuardianChanges(),
		DeparturePolicy:        api.ParentDeparturePolicy(),
		GuardianPolicy:         api.NewParentGuardianPolicy(),
		Fingerprints:           api.SecurityFingerprints{},
		Declarations:           api.ParentDeclarationPolicy(),
		Locales:                api.ParentPortalLocales(),
		Runtime:                api.ParentPortalRuntime{},
		CarePlan:               api.NewParentCarePlanCommands(reposCare.CarePlan(), nil),
		People:                 api.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:           careProfiles(t, db),
		ChildRepo:              reposCare.ParentChildReads(),
		StatusDayRepo:          reposCare.StudentStatusDay,
		StudentRepo:            api.NewParentStudentReads(repos.Student),
		PersonRepo:             api.NewParentPersonReads(repos.Person),
		GuardianProfileRepo:    api.NewParentGuardianProfileReads(repos.GuardianProfile),
		StudentGuardianRepo:    api.NewParentStudentGuardianReads(repos.StudentGuardian),
		FamilyProtectionEvents: api.BindParentFamilyProtectionReads(repos.FamilyProtection),
		ParentRequestShares:    api.BindParentRequestShareRecords(repos.ParentRequestShare),
		ParentRequestEvents:    api.NewParentRequestEvents(repos.ParentRequestEvent),
		ExcusedRequests:        api.NewParentExcusedAbsenceRequests(excused),
		ExcusedRequestRepo:     reposCare.ExcusedAbsenceRequest,
		Settings: excusedApprovalSettings{
			sickRequiresApproval:    true,
			excusedRequiresApproval: true,
		},
		Logger: slog.Default(),

		DataRequests: api.NewParentDataRequestCommands(reposCare.CarePlan()),

		CarePlanPolicy: api.NewParentCarePlanPolicy(),
	})
	// Portal implements the sharing surface at compile time.
	var sharing parentService.RequestSharingService = svc
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
		[]calendar.Date{calendar.TodayDate().AddDays(3)}, "Familienfeier",
		absencerecords.StudentStatusDayExcused, []int64{co.GuardianProfileID})
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
		[]calendar.Date{calendar.TodayDate().AddDays(4)}, "Familienfeier",
		absencerecords.StudentStatusDayExcused, []int64{stranger.GuardianProfileID})
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
		[]calendar.Date{calendar.TodayDate().AddDays(5)}, "Familienfeier",
		absencerecords.StudentStatusDayExcused, nil)
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
