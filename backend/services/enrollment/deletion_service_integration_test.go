package enrollment_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type deletionTestFixture struct {
	t     *testing.T
	db    *bun.DB
	repos *repositories.Factory
	scope testpkg.TenantScope
	actor int64
	phase int64
}

func newDeletionTestFixture(t *testing.T, db *bun.DB, label string) *deletionTestFixture {
	t.Helper()
	f := &deletionTestFixture{t: t, db: db, repos: repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)), scope: testpkg.NewTenantScope(t, db)}
	account := testpkg.CreateTestAccount(t, db, "enrollment-deletion-"+label)
	f.actor = account.ID
	phase := &capability.Phase{
		Name:             fmt.Sprintf("deletion-%s-%d", label, f.scope.TenantID),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: capability.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   capability.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
	}
	phase.TenantID = f.scope.TenantID
	require.NoError(t, enrollmentService.InsertOwnerPhaseForTest(f.scope.Context(), f.repos.Enrollment(), phase))
	f.phase = phase.ID
	return f
}

func (f *deletionTestFixture) request(label string, guardianAccountID *int64) *enrollmentModels.Request {
	f.t.Helper()
	request := &enrollmentModels.Request{
		PhaseID:           f.phase,
		GuardianFirstName: "Test",
		GuardianLastName:  "Guardian",
		GuardianEmail:     fmt.Sprintf("%s-%d@example.invalid", label, f.scope.TenantID),
		GuardianAccountID: guardianAccountID,
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("%s-token-%d", label, f.scope.TenantID),
		SubmittedAt:       time.Now(),
	}
	request.TenantID = f.scope.TenantID
	require.NoError(f.t, enrollmentService.InsertOwnerRequestForTest(f.scope.Context(), f.repos.Enrollment(), request))
	return request
}

func (f *deletionTestFixture) child(requestID int64, label, status string, createdStudentID *int64) *enrollmentService.RequestChild {
	f.t.Helper()
	now := time.Now()
	child := &enrollmentService.RequestChild{
		RequestID:        requestID,
		FirstName:        label,
		LastName:         "Child",
		DateOfBirth:      "2018-04-15",
		CustomData:       map[string]any{},
		Status:           status,
		ActivationMode:   enrollmentModels.ChildActivationScheduled,
		ReviewedAt:       &now,
		CreatedStudentID: createdStudentID,
	}
	child.TenantID = f.scope.TenantID
	require.NoError(f.t, enrollmentService.InsertOwnerChildForTest(f.scope.Context(), f.repos.Enrollment(), child))
	return child
}

func (f *deletionTestFixture) service(auditRepo auditModels.EnrollmentDeletionRepository, requestRepo testutil.EnrollmentDeletionOwner) enrollmentService.EnrollmentDeletionService {
	return testutil.NewEnrollmentDeletionModule(f.sources(auditRepo, requestRepo)).Deletions
}

// sources binds the admin deletion and the retention cleanup the way the
// root does; requestRepo replaces the owner's request lock.
func (f *deletionTestFixture) sources(auditRepo auditModels.EnrollmentDeletionRepository, requestRepo testutil.EnrollmentDeletionOwner) testutil.EnrollmentDeletionSources {
	if auditRepo == nil {
		auditRepo = f.repos.EnrollmentDeletionAudit
	}
	if requestRepo == nil {
		requestRepo = f.repos.Enrollment()
	}
	return testutil.EnrollmentDeletionSources{
		Owner:                 requestRepo,
		Guardians:             f.guardians(),
		CountAuditAdjustments: f.repos.EnrollmentOfferingAdjustment.CountForDeletion,
		CountBookings:         f.repos.CarePlan().CountCareOfferingBookings,
		Audit:                 auditRepo,
		Delivery:              newTestEnrollmentDelivery(f.t, f.db),
		Logger:                slog.New(slog.DiscardHandler),
	}
}

func tenantCall[T any](t *testing.T, db *bun.DB, tenantID int64, fn func(context.Context) (T, error)) (T, error) {
	t.Helper()
	var result T
	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var callErr error
		result, callErr = fn(ctx)
		return callErr
	})
	return result, err
}

func tableCount(t *testing.T, db *bun.DB, table, where string, args ...any) int {
	t.Helper()
	count, err := db.NewSelect().TableExpr(table).Where(where, args...).Count(context.Background())
	require.NoError(t, err)
	return count
}

func TestEnrollmentDeletionOwner_MismatchedRequestPreservesChildSelections(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "mismatched-request")
	request := f.request("target", nil)
	other := f.request("other", nil)
	child := f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)
	offering := &enrollmentModels.CareOffering{PhaseID: f.phase, Name: "Care", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}, IsActive: true, CountsAsCare: true, CountsAsCareSet: true}
	require.NoError(t, enrollmentService.NewCareOfferingRepository(f.repos.CarePlan()).Create(f.scope.Context(), offering))
	selection := &capability.RequestChildOffering{RequestChildID: child.ID, CareOfferingID: offering.ID, SelectedDays: []string{"mon"}}
	require.NoError(t, repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertRequestChildOffering(f.scope.Context(), selection))
	counts, err := f.repos.Enrollment().DeletionChildCounts(f.scope.Context(), other.ID, child.ID)
	require.NoError(t, err)
	require.Zero(t, counts.Offerings)
	require.NoError(t, f.repos.Enrollment().DeleteRequestChildTree(f.scope.Context(), other.ID, child.ID))
	links, err := f.repos.Enrollment().RequestChildOfferingHistory(f.scope.Context(), child.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, selection.ID, links[0].ID)
	stored, err := f.repos.Enrollment().ChildByID(f.scope.Context(), child.ID)
	require.NoError(t, err)
	require.Equal(t, request.ID, stored.RequestID)
}

func TestEnrollmentDeletion_DeleteChildFromMixedRequestPreservesSharedData(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "mixed")
	request := f.request("mixed", nil)
	target := f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)
	remaining := f.child(request.ID, "Approved", enrollmentModels.ChildStatusApproved, nil)

	change := &capability.ChangeRequest{
		RequestID:      request.ID,
		RequestChildID: &target.ID,
		Origin:         capability.ChangeRequestOriginParent,
		BaseSnapshot:   json.RawMessage("{}"), ProposedSnapshot: json.RawMessage("{}"), Diff: json.RawMessage("{}"),
	}
	require.NoError(t, f.repos.Enrollment().InsertChangeRequest(f.scope.Context(), change))
	message := &capability.ChangeRequestMessage{ChangeRequestID: change.ID, AuthorType: capability.ChangeRequestMessageAuthorParent, Body: "remove with child"}
	require.NoError(t, f.repos.Enrollment().InsertChangeRequestMessage(f.scope.Context(), message))
	guardian := &capability.RequestGuardian{RequestID: request.ID, FirstName: "Other", LastName: "Guardian"}
	require.NoError(t, f.repos.Enrollment().CreateRequestGuardian(f.scope.Context(), guardian))
	outbox := enqueueTestEnrollmentEmail(t, db, f.scope.TenantID, request.ID, fmt.Sprintf("mixed-%d", request.ID), map[string]any{})

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).DeleteChild(ctx, request.ID, target.ID, f.actor, "Fehlerhafte Teilanmeldung")
	})
	require.NoError(t, err)
	require.NotNil(t, impact)
	assert.False(t, impact.DeletesRequest)
	assert.Equal(t, 1, impact.Counts.RequestChildren)
	assert.Equal(t, 1, impact.Counts.ChangeRequests)
	assert.Equal(t, 1, impact.Counts.ChangeRequestMessages)

	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Zero(t, tableCount(t, db, "enrollment.request_children", "id = ?", target.ID))
	assert.Equal(t, 1, tableCount(t, db, "enrollment.request_children", "id = ?", remaining.ID))
	assert.Zero(t, tableCount(t, db, "enrollment.change_requests", "id = ?", change.ID))
	assert.Zero(t, tableCount(t, db, "enrollment.change_request_messages", "id = ?", message.ID))
	assert.Equal(t, 1, tableCount(t, db, "enrollment.request_guardians", "id = ?", guardian.ID))
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ?", outbox.ID))
	_, err = f.repos.Enrollment().RequestByToken(f.scope.Context(), request.StatusToken, false)
	require.NoError(t, err)
	assert.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ? AND child_id = ?", f.scope.TenantID, request.ID, target.ID))
}

func TestEnrollmentDeletion_DeleteApprovedChildAfterStudentWasRemoved(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "approved-orphan")
	request := f.request("approved-orphan", nil)
	target := f.child(request.ID, "ApprovedOrphan", enrollmentModels.ChildStatusApproved, nil)
	remaining := f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).DeleteChild(ctx, request.ID, target.ID, f.actor, "Gelöschtes Testkind bereinigen")
	})
	require.NoError(t, err)
	assert.False(t, impact.DeletesRequest)
	assert.Zero(t, tableCount(t, db, "enrollment.request_children", "id = ?", target.ID))
	assert.Equal(t, 1, tableCount(t, db, "enrollment.request_children", "id = ?", remaining.ID))
	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
}

func TestEnrollmentDeletion_DeleteRequestCleansDependenciesAndPreservesPeople(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "request")
	guardianAccount := testpkg.CreateTestAccount(t, db, "preserved-parent")
	email := fmt.Sprintf("guardian-%d@example.invalid", f.scope.TenantID)
	profile := &userModels.GuardianProfile{FirstName: "Preserved", LastName: "Guardian", Email: &email, AccountID: &guardianAccount.ID, HasAccount: true, PreferredContactMethod: "email", LanguagePreference: "de"}
	profile.SetTenantID(f.scope.TenantID)
	require.NoError(t, f.repos.GuardianProfile.Create(f.scope.Context(), profile))
	request := f.request("whole", &guardianAccount.ID)
	child := f.child(request.ID, "ApprovedOrphan", enrollmentModels.ChildStatusApproved, nil)
	coGuardian := &capability.RequestGuardian{RequestID: request.ID, FirstName: "Preserved", LastName: "Guardian", GuardianProfileID: &profile.ID}
	require.NoError(t, f.repos.Enrollment().CreateRequestGuardian(f.scope.Context(), coGuardian))

	offering := &enrollmentModels.CareOffering{PhaseID: f.phase, Name: "Test care", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}, IsActive: true, CountsAsCare: true, CountsAsCareSet: true, AutoAddGradeLevels: []int{}, SelectionRule: enrollmentModels.SelectionRuleOptional}
	require.NoError(t, enrollmentService.NewCareOfferingRepository(f.repos.CarePlan()).Create(f.scope.Context(), offering))
	require.NoError(t, f.repos.Enrollment().RecordSubmittedOfferingChoices(f.scope.Context(), child.ID, []capability.SubmittedOfferingChoice{
		{CareOfferingID: offering.ID, SelectedDays: []string{"mon"}},
	}))
	start, until := timezone.NewDate(2026, 9, 1), timezone.NewDate(2027, 8, 1)
	require.NoError(t, requestTestBookingCommands().RecordCareBookings(f.scope.Context(), child.ID, []enrollmentService.CareBookingInput{
		{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}, ValidFrom: &start, ValidUntil: &until},
	}))
	bookings, err := f.repos.CarePlan().CareOfferingBookingHistory(f.scope.Context(), []int64{child.ID})
	require.NoError(t, err)
	require.Len(t, bookings, 1)
	childOffering := bookings[0]
	change := &capability.ChangeRequest{RequestID: request.ID, RequestChildID: &child.ID, Origin: capability.ChangeRequestOriginParent, BaseSnapshot: json.RawMessage("{}"), ProposedSnapshot: json.RawMessage("{}"), Diff: json.RawMessage("{}")}
	require.NoError(t, f.repos.Enrollment().InsertChangeRequest(f.scope.Context(), change))
	message := &capability.ChangeRequestMessage{ChangeRequestID: change.ID, AuthorType: capability.ChangeRequestMessageAuthorStaff, AuthorAccountID: &f.actor, Body: "dependent message"}
	require.NoError(t, f.repos.Enrollment().InsertChangeRequestMessage(f.scope.Context(), message))
	invite := &capability.LateInvite{PhaseID: f.phase, TokenHash: fmt.Sprintf("invite-%d", f.scope.TenantID), GuardianEmail: request.GuardianEmail, ExpiresAt: time.Now().Add(time.Hour), CreatedBy: f.actor}
	require.NoError(t, f.repos.Enrollment().InsertLateInvite(f.scope.Context(), invite))
	require.NoError(t, f.repos.Enrollment().MarkLateInviteUsed(f.scope.Context(), invite.ID, request.ID, time.Now()))
	outbox := enqueueTestEnrollmentEmail(t, db, f.scope.TenantID, request.ID, fmt.Sprintf("request-%d", request.ID), map[string]any{"request_id": request.ID})
	student := testpkg.CreateTestStudentForTenant(t, db, f.scope.TenantID, "Adjustment", "Student", "1a")
	adjustment := &auditModels.EnrollmentOfferingAdjustment{RequestID: request.ID, RequestChildID: child.ID, StudentID: student.ID, ActorAccountID: f.actor, ActorRole: "admin", Reason: "test adjustment", Before: json.RawMessage(`{}`), After: json.RawMessage(`{}`), ChangedAt: time.Now()}
	require.NoError(t, f.repos.EnrollmentOfferingAdjustment.Create(f.scope.Context(), adjustment))

	preview, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).PreviewRequest(ctx, request.ID)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, preview.PreservedGuardianProfiles)
	assert.Equal(t, 1, preview.PreservedParentAccounts)
	assert.Equal(t, 1, preview.UnlinkedGuardianProfiles)
	assert.Equal(t, 1, preview.ParentAccountsWithoutStudents)
	assert.Equal(t, 1, preview.Counts.EmailOutbox)
	assert.Equal(t, 1, preview.Counts.OfferingAdjustments)
	assert.Equal(t, 1, preview.Counts.RequestChildOfferings)

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).DeleteRequest(ctx, request.ID, f.actor, "Genehmigte Testanmeldung bereinigen")
	})
	require.NoError(t, err)
	assert.Equal(t, preview.Counts, impact.Counts)
	assert.Zero(t, tableCount(t, db, "enrollment.request_child_offering_selections", "tenant_id = ? AND request_child_id = ?", f.scope.TenantID, child.ID))
	assert.Zero(t, tableCount(t, db, "enrollment.care_offering_bookings", "tenant_id = ? AND request_child_id = ?", f.scope.TenantID, child.ID))
	for _, check := range []struct {
		table string
		id    int64
	}{
		{"enrollment.requests", request.ID},
		{"enrollment.request_children", child.ID},
		{"enrollment.care_offering_bookings", childOffering.ID},
		{"enrollment.request_guardians", coGuardian.ID},
		{"enrollment.change_requests", change.ID},
		{"enrollment.change_request_messages", message.ID},
		{"enrollment.late_invites", invite.ID},
		{"audit.enrollment_offering_adjustments", adjustment.ID},
	} {
		assert.Zero(t, tableCount(t, db, check.table, "id = ?", check.id), check.table)
	}
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ? AND status = 'cancelled'", outbox.ID))
	assert.Equal(t, 1, tableCount(t, db, "users.student_profiles", "id = ?", student.ID))
	assert.Equal(t, 1, tableCount(t, db, "users.guardian_profiles", "id = ?", profile.ID))
	assert.Equal(t, 1, tableCount(t, db, "auth.accounts", "id = ?", guardianAccount.ID))
	_, err = f.repos.Enrollment().RequestByToken(f.scope.Context(), request.StatusToken, false)
	require.Error(t, err)
	assert.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ?", f.scope.TenantID, request.ID))
}

func TestEnrollmentDeletion_DeleteLastChildAlsoDeletesRequest(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "last-child")
	request := f.request("last-child", nil)
	child := f.child(request.ID, "Withdrawn", enrollmentModels.ChildStatusWithdrawn, nil)

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).DeleteChild(ctx, request.ID, child.ID, f.actor, "Letztes zurückgezogenes Kind")
	})
	require.NoError(t, err)
	assert.True(t, impact.DeletesRequest)
	assert.Equal(t, 1, impact.Counts.Requests)
	assert.Zero(t, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ? AND child_id = ?", f.scope.TenantID, request.ID, child.ID))
}

func TestEnrollmentDeletion_BlocksExistingStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "student-block")
	student := testpkg.CreateTestStudentForTenant(t, db, f.scope.TenantID, "Existing", "Student", "2a")
	request := f.request("student-block", nil)
	f.child(request.ID, "Approved", enrollmentModels.ChildStatusApproved, &student.ID)

	preview, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).PreviewRequest(ctx, request.ID)
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, preview.BlockingStudentIDs)
	_, err = tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(nil, nil).DeleteRequest(ctx, request.ID, f.actor, "Darf nicht gelöscht werden")
	})
	require.ErrorIs(t, err, enrollmentService.ErrEnrollmentDeletionStudentExists)
	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Equal(t, 1, tableCount(t, db, "users.student_profiles", "id = ?", student.ID))
	assert.Zero(t, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ?", f.scope.TenantID))
}

type failingDeletionAudit struct{ err error }

func (a failingDeletionAudit) Create(context.Context, *auditModels.EnrollmentDeletion) error {
	return a.err
}

func TestEnrollmentDeletion_AuditFailureRollsBackAllDeletes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "rollback")
	request := f.request("rollback", nil)
	f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)
	outbox := enqueueTestEnrollmentEmail(t, db, f.scope.TenantID, request.ID, fmt.Sprintf("rollback-%d", request.ID), map[string]any{})

	expected := errors.New("audit unavailable")
	_, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return f.service(failingDeletionAudit{err: expected}, nil).DeleteRequest(ctx, request.ID, f.actor, "Rollback prüfen")
	})
	require.ErrorIs(t, err, expected)
	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ?", outbox.ID))
}

func TestEnrollmentDeletion_RLSDeniesOtherTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := newDeletionTestFixture(t, db, "rls-a")
	tenantB := newDeletionTestFixture(t, db, "rls-b")
	requestB := tenantB.request("rls-target", nil)
	tenantB.child(requestB.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)

	_, err := tenantCall(t, db, tenantA.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
		return tenantA.service(nil, nil).PreviewRequest(ctx, requestB.ID)
	})
	require.ErrorIs(t, err, enrollmentService.ErrEnrollmentDeletionNotFound)
	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", requestB.ID))
}

type deletionLockSignalRepository struct {
	testutil.EnrollmentDeletionOwner
	targetID int64
	started  chan struct{}
	once     sync.Once
}

func (r *deletionLockSignalRepository) RequestByID(ctx context.Context, id int64, forUpdate bool) (*capability.Request, error) {
	if id == r.targetID {
		r.once.Do(func() { close(r.started) })
	}
	return r.EnrollmentDeletionOwner.RequestByID(ctx, id, forUpdate)
}

func TestEnrollmentDeletion_ConcurrentDecisionIsRecheckedUnderLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "concurrent")
	request := f.request("concurrent", nil)
	child := f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)

	decisionTx, err := db.BeginTx(f.scope.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = decisionTx.Rollback() }()
	_, err = decisionTx.ExecContext(f.scope.Context(), `SELECT id FROM enrollment.requests WHERE id = ? FOR UPDATE`, request.ID)
	require.NoError(t, err)
	_, err = decisionTx.ExecContext(f.scope.Context(), `UPDATE enrollment.request_children SET status = ?, reviewed_at = ? WHERE id = ?`, enrollmentModels.ChildStatusUnderReview, time.Now(), child.ID)
	require.NoError(t, err)

	started := make(chan struct{})
	requestRepo := &deletionLockSignalRepository{EnrollmentDeletionOwner: f.repos.Enrollment(), targetID: request.ID, started: started}
	finished := make(chan error, 1)
	go func() {
		_, deleteErr := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*capability.DeletionImpact, error) {
			return f.service(nil, requestRepo).DeleteChild(ctx, request.ID, child.ID, f.actor, "Parallelentscheidung prüfen")
		})
		finished <- deleteErr
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("deletion did not reach the request lock")
	}
	require.NoError(t, decisionTx.Commit())
	select {
	case err = <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("deletion did not finish after concurrent decision committed")
	}
	require.ErrorIs(t, err, enrollmentService.ErrEnrollmentDeletionNotAllowed)
	assert.Equal(t, 1, tableCount(t, db, "enrollment.request_children", "id = ?", child.ID))
}

func TestRejectedEnrollmentCleanup_WritesSystemDeletionAudit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "retention-audit")
	request := f.request("retention-audit", nil)
	child := f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)
	outbox := enqueueTestEnrollmentEmail(t, db, f.scope.TenantID, request.ID, fmt.Sprintf("cleanup-%d", request.ID), map[string]any{})
	expectedOutboxRows := tableCount(t, db, "platform.email_outbox", "id = ?", outbox.ID)
	oldReview := time.Now().Add(-100 * 24 * time.Hour)
	_, err := db.NewUpdate().TableExpr("enrollment.request_children").Set("reviewed_at = ?", oldReview).Where("id = ?", child.ID).Exec(context.Background())
	require.NoError(t, err)

	sources := f.sources(nil, nil)
	sources.Settings = cleanupRetentionSettings{days: 90}
	cleaner := testutil.NewEnrollmentDeletionModule(sources).Cleanup
	result, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (capability.RejectedEnrollmentCleanupResult, error) {
		return cleaner.CleanupRejectedEnrollments(ctx)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.DeletedRequests)
	assert.EqualValues(t, expectedOutboxRows, result.DeletedOutboxRows)
	assert.Zero(t, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ? AND status = 'cancelled'", outbox.ID))
	assert.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ? AND actor_type = 'system' AND actor_account_id IS NULL", f.scope.TenantID, request.ID))
	retry, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (capability.RejectedEnrollmentCleanupResult, error) {
		return cleaner.CleanupRejectedEnrollments(ctx)
	})
	require.NoError(t, err)
	require.Equal(t, capability.RejectedEnrollmentCleanupResult{}, retry)
	require.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ?", f.scope.TenantID, request.ID), "retry must not append a second deletion audit")
	observed, err := json.MarshalIndent(struct {
		FirstRun capability.RejectedEnrollmentCleanupResult `json:"first_run"`
		Retry    capability.RejectedEnrollmentCleanupResult `json:"retry"`
	}{result, retry}, "", "  ")
	require.NoError(t, err)
	expected, err := os.ReadFile("testdata/rejected_cleanup.golden")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(observed))
}

func (f *deletionTestFixture) guardians() deletionTestGuardians {
	f.t.Helper()
	people, err := repositories.NewPeopleDirectory(f.db)
	require.NoError(f.t, err)
	return deletionTestGuardians{
		byAccount: func(ctx context.Context, ids []int64) ([]enrollmentTest.DirectoryGuardian, error) {
			values, err := people.ListGuardiansByAccount(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]enrollmentTest.DirectoryGuardian, 0, len(values))
			for _, value := range values {
				result = append(result, enrollmentTest.DirectoryGuardian{ID: value.ID, AccountID: value.AccountID})
			}
			return result, nil
		},
		byID: func(ctx context.Context, ids []int64) ([]enrollmentTest.DirectoryGuardian, error) {
			values, err := people.ListGuardiansByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]enrollmentTest.DirectoryGuardian, 0, len(values))
			for _, value := range values {
				result = append(result, enrollmentTest.DirectoryGuardian{ID: value.ID, AccountID: value.AccountID})
			}
			return result, nil
		},
		count: people.CountGuardianLinks,
	}
}

type deletionTestGuardians struct {
	byAccount func(context.Context, []int64) ([]enrollmentTest.DirectoryGuardian, error)
	byID      func(context.Context, []int64) ([]enrollmentTest.DirectoryGuardian, error)
	count     func(context.Context, []int64) (map[int64]int, error)
}

func (d deletionTestGuardians) ListGuardiansByAccount(ctx context.Context, ids []int64) ([]enrollmentTest.DirectoryGuardian, error) {
	return d.byAccount(ctx, ids)
}
func (d deletionTestGuardians) ListGuardiansByID(ctx context.Context, ids []int64) ([]enrollmentTest.DirectoryGuardian, error) {
	return d.byID(ctx, ids)
}
func (d deletionTestGuardians) CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error) {
	return d.count(ctx, ids)
}
