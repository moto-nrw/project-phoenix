package parentportal_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// #4780: pickup permissions, one-day collectors and the way home alone carry
// a simple electronic signature with a drawn signature; the OGS confirms the
// request. The clocks are pinned: today is Tuesday 2026-11-03, the requested
// day Tuesday 2026-11-10.
var (
	pickupPermissionToday = calendar.NewDate(2026, time.November, 3)
	pickupPermissionDay   = calendar.NewDate(2026, time.November, 10)
)

func pickupPermissionNow() time.Time {
	return pickupPermissionToday.BerlinMidnight().Add(12 * time.Hour)
}

type pickupPermissionFixture struct {
	svc      *parentService.Portal
	db       *bun.DB
	requests carerequests.Service
	care     careplan.Capability
}

func buildPickupPermissionService(t *testing.T) pickupPermissionFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	timetableOwner := owners.MustNewUnobservedTimetable(testpkg.NewDatabase(t, db))
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	_, schedules, sfServices, caller, persons, _, err := api.NewTestCareRequests(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	permissions, err := careplanCompose.NewGuardianPickupPermissions(db, func(careplanCompose.Observation) {})
	require.NoError(t, err)
	requests := owners.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		repos.CarePlan(), persons, schedules.Arrivals, schedules.Pickups, presence,
		newPickupExcusal(t, db, repos.CarePlan(),
			newPickupBaselineService(repos.CarePlan(), approvedOfferingProjection(t)),
			timetableOwner, false,
		),
		repos.CarePlan(), caller, nil, nil,
		testpkg.NewRequestReviewPolicy(caller, reposPeople.Student.FindByIDForUpdate), testpkg.PermissionsFromContext,
		nil, slog.Default(), sfServices.StudentAudit,
		owners.WithCareRequestToday(func() calendar.Date { return pickupPermissionToday }),
		owners.WithPickupPermissions(permissions),
	)
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:     owners.ParentGuardianChanges(),
		Signatures:          owners.ParentDeclarationSignatures{Ledger: owners.SignatureLedger()},
		DeparturePolicy:     owners.ParentDeparturePolicy(),
		GuardianPolicy:      owners.NewParentGuardianPolicy(),
		Fingerprints:        owners.SecurityFingerprints{},
		Declarations:        owners.ParentDeclarationPolicy(),
		Locales:             owners.ParentPortalLocales(),
		Runtime:             owners.ParentPortalRuntime{},
		Now:                 pickupPermissionNow,
		CarePlan:            owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:              owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:        careProfiles(t, db),
		ChildRepo:           repos.ParentChildReads(),
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         owners.NewParentStudentReads(reposPeople.Student),
		StudentGuardianRepo: owners.NewParentStudentGuardianReads(reposPeople.StudentGuardian),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(reposPeople.GuardianProfile),
		GuardianPhoneRepo:   owners.NewParentGuardianPhoneReads(reposPeople.GuardianPhoneNumber),
		GuardianInvites:     &stubInvites{},
		GuardianInvitations: guardianInvitationReads{db: db},
		CareExceptions:      owners.NewParentCareExceptions(repos.CarePlan()),
		Attendance:          parentAttendance(t, db),
		CareRequests:        requests,
		Settings: parentSettingsStub{boolValues: map[string]bool{
			parentService.KeyParentPickupChangeEnabled:       true,
			parentService.KeyParentGuardianManagementEnabled: true,
			parentService.KeyParentCareModeRequestEnabled:    true,
		}, stringValues: map[string]string{
			parentService.KeyGuardianParentInviteMode: parentService.ParentInviteModeDisabled,
		}},
		Logger:         slog.Default(),
		DataRequests:   owners.NewParentDataRequestCommands(repos.CarePlan()),
		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
	return pickupPermissionFixture{svc: svc, db: db, requests: requests, care: repos.CarePlan()}
}

// testSignature is a drawn signature: a small PNG with one visible stroke.
func testSignature(t *testing.T) *parentService.PickupSignatureInput {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 5; x < 35; x++ {
		img.Set(x, 10, color.Black)
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return &parentService.PickupSignatureInput{Method: "drawn", ImagePNG: base64.StdEncoding.EncodeToString(buf.Bytes())}
}

func staffDecisionContext(t *testing.T, db *bun.DB, tenantID int64) (context.Context, int64) {
	t.Helper()
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Review")
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
	return testpkg.WithAuthenticatedContext(ctx, testpkg.Claims{ID: int(staffAccount.ID)}, []string{"admin:*"}), staffAccount.ID
}

func approveCareRequest(t *testing.T, f pickupPermissionFixture, tenantID, requestID int64) {
	t.Helper()
	staffCtx, staffID := staffDecisionContext(t, f.db, tenantID)
	token := ""
	err := testpkg.WithTenantTx(t, staffCtx, f.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := f.requests.Decide(txCtx, carerequests.DecideInput{
			RequestID: requestID, Approve: true, ReviewedBy: staffID, ExpectedImpactToken: &token,
		})
		return decideErr
	})
	require.NoError(t, err)
}

// storedSignatureHash reads the purpose, child and document checksum of a
// stored signature.
func storedSignature(t *testing.T, db *bun.DB, id int64) (purpose string, studentID int64, documentSHA string) {
	t.Helper()
	err := db.NewSelect().TableExpr("audit.signatures").ColumnExpr("purpose, student_id, document_sha256").
		Where("id = ?", id).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &purpose, &studentID, &documentSHA)
	require.NoError(t, err)
	return purpose, studentID, documentSHA
}

func sha256Hex(text string) string {
	return owners.SecurityFingerprints{}.Content([]byte(text))
}

func pickupWeekdays(t *testing.T, db *bun.DB, studentID, profileID int64) (bool, []int) {
	t.Helper()
	var row struct {
		CanPickup bool  `bun:"can_pickup"`
		Weekdays  []int `bun:"pickup_weekdays,array"`
	}
	err := db.NewSelect().TableExpr("users.student_guardian_relationships AS r").
		Join("JOIN users.student_guardian_pickup_permissions AS p ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id").
		ColumnExpr("p.can_pickup, p.pickup_weekdays").
		Where("r.student_id = ? AND r.guardian_profile_id = ?", studentID, profileID).
		Scan(testpkg.WithPackageTenantRuntime(context.Background()), &row)
	require.NoError(t, err)
	return row.CanPickup, row.Weekdays
}

func TestPickupPermissionRequest_SignedThenConfirmedGrantsWeekdays(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-weekdays")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	created, err := f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, contactID, parentService.PickupPermissionInput{
		Weekdays: []int{4, 2, 2}, Signature: testSignature(t),
	})
	require.NoError(t, err)
	assert.Equal(t, carerequests.StatusPending, created.Status)
	assert.Equal(t, carerequests.KindPickupPermission, created.RequestKind)

	permission, err := carerequests.ParsePickupPermission(created.Payload)
	require.NoError(t, err)
	assert.Equal(t, []int{2, 4}, permission.Weekdays, "weekdays are sorted and deduplicated")
	require.NotNil(t, permission.Signature)
	assert.Contains(t, permission.Signature.Statement, "dienstags und donnerstags")
	purpose, signedStudent, documentSHA := storedSignature(t, f.db, permission.Signature.SignatureID)
	assert.Equal(t, "pickup_permission", purpose)
	assert.Equal(t, chain.StudentID, signedStudent)
	assert.Equal(t, sha256Hex(permission.Signature.Statement), documentSHA, "the signature binds the exact wording")

	canPickup, _ := pickupWeekdays(t, f.db, chain.StudentID, contactID)
	assert.False(t, canPickup, "the request alone grants nothing")

	approveCareRequest(t, f, chain.TenantID, created.ID)

	canPickup, weekdays := pickupWeekdays(t, f.db, chain.StudentID, contactID)
	assert.True(t, canPickup, "the confirmed request grants the pickup permission")
	assert.Equal(t, []int{2, 4}, weekdays)

	guardians, err := f.svc.ListChildGuardians(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	for _, g := range guardians {
		if g.GuardianProfileID == contactID {
			assert.True(t, g.CanPickup)
			assert.Equal(t, []int{2, 4}, g.PickupWeekdays, "the child profile reads the weekday limit")
		}
	}

	proof, err := f.svc.PickupRequestProof(ctx, chain.AccountID, chain.StudentID, created.ID)
	require.NoError(t, err)
	assert.True(t, proof.Intact)
	require.NotNil(t, proof.Signature)
	assert.NotEmpty(t, proof.Signature.PNG)
	assert.Equal(t, permission.Signature.Statement, proof.Statement)
}

func TestPickupPermissionCorrectionRestoresThePreviousPermission(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-correction")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	created, err := f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, contactID, parentService.PickupPermissionInput{
		Weekdays: []int{2, 4}, Signature: testSignature(t),
	})
	require.NoError(t, err)
	approveCareRequest(t, f, chain.TenantID, created.ID)

	rows, err := f.svc.ListPickupPermissionRequests(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	current := rows[0]
	staffCtx, staffID := staffDecisionContext(t, f.db, chain.TenantID)
	corrector, ok := f.requests.(interface {
		Correct(context.Context, int64, bool, string, string, int64) error
	})
	require.True(t, ok)
	require.NoError(t, testpkg.WithTenantTx(t, staffCtx, f.db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return corrector.Correct(txCtx, created.ID, false, careplan.ParentRequestVersion(current.UpdatedAt), "Doch nicht", staffID)
	}))

	canPickup, weekdays := pickupWeekdays(t, f.db, chain.StudentID, contactID)
	assert.False(t, canPickup)
	assert.Empty(t, weekdays)
}

func TestPickupPermissionRequestsListQueryBudget(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	for _, name := range []string{"eins", "zwei", "drei"} {
		contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-budget-"+name)
		_, err := f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, contactID, parentService.PickupPermissionInput{Signature: testSignature(t)})
		require.NoError(t, err)
	}

	counter := testpkg.CaptureQueriesForContext(t, f.db)
	ctx = counter.Context(ctx)
	rows, err := f.svc.ListPickupPermissionRequests(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
	testpkg.AssertQueryBudget(t, "workflows.parentportal.pickup_permission_requests.list", counter.Queries())
}

func TestPickupPermissionRequest_RefusesWithoutSignatureOrPermission(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-refuse")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, contactID, parentService.PickupPermissionInput{})
	require.ErrorIs(t, err, parentService.ErrPickupSignatureRequired)

	// A guardian of the child without parent_portal.pickup.manage.
	viewerID := linkAccountGuardian(t, f.db, chain.StudentID, "viewer-4780", map[string]interface{}{
		"parent_portal.access":        true,
		"parent_portal.guardian.edit": true,
	})
	_, err = f.svc.SubmitPickupPermissionRequest(ctx, accountIDForProfile(t, f.db, viewerID), chain.StudentID, contactID, parentService.PickupPermissionInput{
		Signature: testSignature(t),
	})
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)

	// An account holder never receives it from another parent.
	_, err = f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, viewerID, parentService.PickupPermissionInput{
		Signature: testSignature(t),
	})
	require.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)

	rows, err := f.svc.ListPickupPermissionRequests(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "no refused attempt left a request")
}

func TestPickupPermissionRequest_OnlyOwnChildrenOfTheSchool(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	other := testpkg.CreateTestParentGuardianChain(t, f.db)
	contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-own")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	// Another family's guardian cannot ask for this child.
	_, err := f.svc.SubmitPickupPermissionRequest(ctx, other.AccountID, chain.StudentID, contactID, parentService.PickupPermissionInput{
		Signature: testSignature(t),
	})
	require.Error(t, err, "membership in the school is not a relationship to this child")

	// The guardian cannot name a contact of another family's child.
	otherContact := linkContactOnlyGuardian(t, f.db, other.StudentID, "oma-4780-other")
	_, err = f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, otherContact, parentService.PickupPermissionInput{
		Signature: testSignature(t),
	})
	require.ErrorIs(t, err, parentService.ErrGuardianNotLinked)

	// A child of another school is not reachable either.
	foreignTenant, _ := testpkg.CreateTestTenant(t, f.db)
	foreignChild := testpkg.CreateTestStudentForTenant(t, f.db, foreignTenant, "Fremd", "Kind", "2b")
	_, err = f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, foreignChild.ID, contactID, parentService.PickupPermissionInput{
		Signature: testSignature(t),
	})
	require.Error(t, err)

	rows, err := f.svc.ListPickupPermissionRequests(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestDayPickupPerson_SignedConfirmedAndOnlyThatDay(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-day")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := f.svc.SubmitPickupChange(ctx, chain.AccountID, chain.StudentID, parentService.PickupChangeSubmission{
		Date: pickupPermissionDay, PickupPersonID: contactID, Reason: "Oma holt ab",
	})
	require.ErrorIs(t, err, parentService.ErrPickupSignatureRequired)

	_, err = f.svc.SubmitPickupChange(ctx, chain.AccountID, chain.StudentID, parentService.PickupChangeSubmission{
		Date: pickupPermissionDay, PickupPersonID: contactID, DepartureMode: "alone", Reason: "Oma holt ab", Signature: testSignature(t),
	})
	require.ErrorIs(t, err, parentService.ErrInvalidCareRequestPayload, "a collected child does not walk home alone")

	created, err := f.svc.SubmitPickupChange(ctx, chain.AccountID, chain.StudentID, parentService.PickupChangeSubmission{
		Date: pickupPermissionDay, PickupPersonID: contactID, Reason: "Oma holt ab", Signature: testSignature(t),
	})
	require.NoError(t, err)
	terms := carerequests.StoredPickupTerms(created.Payload)
	require.NotNil(t, terms)
	assert.NotEmpty(t, terms.PickupPersonName)
	signed := carerequests.StoredSignature(created.Payload)
	require.NotNil(t, signed)
	assert.Contains(t, signed.Statement, "am Dienstag, 10.11.2026")

	approveCareRequest(t, f, chain.TenantID, created.ID)

	tenantCtx := tenant.WithTenantID(ctx, chain.TenantID)
	onDay, err := f.care.ListPickupNotes(tenantCtx, careplan.StudentScheduleFilter{StudentIDs: []int64{chain.StudentID}, Date: careplan.Date(pickupPermissionDay)})
	require.NoError(t, err)
	require.Len(t, onDay, 1, "the confirmed collector is noted for that day")
	assert.Contains(t, onDay[0].Content, terms.PickupPersonName)
	nextDay, err := f.care.ListPickupNotes(tenantCtx, careplan.StudentScheduleFilter{StudentIDs: []int64{chain.StudentID}, Date: careplan.Date(pickupPermissionDay.AddDays(1))})
	require.NoError(t, err)
	assert.Empty(t, nextDay, "the collector is not noted on the next day")

	canPickup, _ := pickupWeekdays(t, f.db, chain.StudentID, contactID)
	assert.False(t, canPickup, "a one-day collector does not become a standing pickup permission")
}

func TestDayPickupPerson_MustBeAContactOfTheChild(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	other := testpkg.CreateTestParentGuardianChain(t, f.db)
	otherContact := linkContactOnlyGuardian(t, f.db, other.StudentID, "oma-4780-stranger")

	_, err := f.svc.SubmitPickupChange(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, parentService.PickupChangeSubmission{
		Date: pickupPermissionDay, PickupPersonID: otherContact, Reason: "Fremde Person", Signature: testSignature(t),
	})
	require.ErrorIs(t, err, parentService.ErrPickupPersonNotLinked)
}

func TestWalkingHomeAloneNeedsASignature(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := f.svc.SubmitPickupChange(ctx, chain.AccountID, chain.StudentID, parentService.PickupChangeSubmission{
		Date: pickupPermissionDay, DepartureMode: "alone", Reason: "Geht allein",
	})
	require.ErrorIs(t, err, parentService.ErrPickupSignatureRequired)

	_, err = f.svc.CreateCareScheduleRequest(ctx, chain.AccountID, chain.StudentID, map[string]any{"weekdays": []any{
		map[string]any{"weekday": 2, "mode": "alone"},
	}})
	require.ErrorIs(t, err, parentService.ErrPickupSignatureRequired, "a weekday alone needs a signature too")

	created, err := f.svc.SubmitPickupChange(ctx, chain.AccountID, chain.StudentID, parentService.PickupChangeSubmission{
		Date: pickupPermissionDay, DepartureMode: "alone", Reason: "Geht allein", Signature: testSignature(t),
	})
	require.NoError(t, err)
	signed := carerequests.StoredSignature(created.Payload)
	require.NotNil(t, signed)
	assert.Contains(t, signed.Statement, "allein nach Hause")
}

func TestPickupRequestProof_OnlyForTheSigningGuardian(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	contactID := linkContactOnlyGuardian(t, f.db, chain.StudentID, "oma-4780-proof")
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	created, err := f.svc.SubmitPickupPermissionRequest(ctx, chain.AccountID, chain.StudentID, contactID, parentService.PickupPermissionInput{
		Signature: testSignature(t),
	})
	require.NoError(t, err)

	coGuardian := linkAccountGuardian(t, f.db, chain.StudentID, "co-4780-proof", map[string]interface{}{
		"parent_portal.access":        true,
		"parent_portal.pickup.manage": true,
	})
	_, err = f.svc.PickupRequestProof(ctx, accountIDForProfile(t, f.db, coGuardian), chain.StudentID, created.ID)
	require.ErrorIs(t, err, parentService.ErrPickupProofNotFound, "nobody sees another person's signature in the portal")

	proof, err := f.svc.PickupRequestProof(ctx, chain.AccountID, chain.StudentID, created.ID)
	require.NoError(t, err)
	assert.True(t, proof.Intact)
	assert.Contains(t, proof.Statement, "an allen Tagen")
}

func TestCreateGuardianContact_PickupBecomesSignedRequest(t *testing.T) {
	t.Parallel()

	f := buildPickupPermissionService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, f.db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	input := parentService.CreateGuardianContactInput{
		Contact:          parentService.GuardianContactInput{FirstName: "Erika", LastName: "Muster"},
		RelationshipType: "relative",
		CanPickup:        true,
		PickupWeekdays:   []int{2},
	}

	_, err := f.svc.CreateGuardianContact(ctx, chain.AccountID, chain.StudentID, input)
	require.ErrorIs(t, err, parentService.ErrPickupSignatureRequired)

	input.PickupSignature = testSignature(t)
	created, err := f.svc.CreateGuardianContact(ctx, chain.AccountID, chain.StudentID, input)
	require.NoError(t, err)
	assert.False(t, created.CanPickup, "the permission waits for the OGS")

	var role string
	require.NoError(t, f.db.NewSelect().TableExpr("users.student_guardian_relationships").ColumnExpr("guardian_role").
		Where("student_id = ? AND guardian_profile_id = ?", chain.StudentID, created.GuardianProfileID).
		Scan(ctx, &role))
	assert.Equal(t, "pickup_only", role, "the role follows what the family asks for")

	rows, err := f.svc.ListPickupPermissionRequests(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	permission, err := carerequests.ParsePickupPermission(rows[0].Payload)
	require.NoError(t, err)
	assert.Equal(t, created.GuardianProfileID, permission.GuardianProfileID)
	assert.Equal(t, []int{2}, permission.Weekdays)
	assert.Equal(t, carerequests.StatusPending, rows[0].Status)
}
