package enrollment_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

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
	f := &deletionTestFixture{t: t, db: db, repos: repositories.NewFactory(db), scope: testpkg.NewTenantScope(t, db)}
	account := testpkg.CreateTestAccount(t, db, "enrollment-deletion-"+label)
	f.actor = account.ID
	phase := &enrollmentModels.Phase{
		Name:             fmt.Sprintf("deletion-%s-%d", label, f.scope.TenantID),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
		IsActive:         true,
	}
	phase.SetTenantID(f.scope.TenantID)
	require.NoError(t, f.repos.Phase.Create(f.scope.Context(), phase))
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
	request.SetTenantID(f.scope.TenantID)
	require.NoError(f.t, f.repos.Request.Create(f.scope.Context(), request))
	return request
}

func (f *deletionTestFixture) child(requestID int64, label, status string, createdStudentID *int64) *enrollmentModels.RequestChild {
	f.t.Helper()
	now := time.Now()
	child := &enrollmentModels.RequestChild{
		RequestID:        requestID,
		FirstName:        label,
		LastName:         "Child",
		DateOfBirth:      timezone.NewDate(2018, 4, 15),
		CustomData:       map[string]any{},
		Status:           status,
		ActivationMode:   enrollmentModels.ChildActivationScheduled,
		ReviewedAt:       &now,
		CreatedStudentID: createdStudentID,
	}
	child.SetTenantID(f.scope.TenantID)
	require.NoError(f.t, f.repos.RequestChild.Create(f.scope.Context(), child))
	return child
}

func (f *deletionTestFixture) service(auditRepo auditModels.EnrollmentDeletionRepository, requestRepo enrollmentModels.RequestRepository) enrollmentService.EnrollmentDeletionService {
	if auditRepo == nil {
		auditRepo = f.repos.EnrollmentDeletionAudit
	}
	if requestRepo == nil {
		requestRepo = f.repos.Request
	}
	return enrollmentService.NewEnrollmentDeletionService(
		requestRepo,
		f.repos.RequestChild,
		f.repos.EnrollmentDeletion,
		auditRepo,
		f.db,
		slog.New(slog.DiscardHandler),
		newTestEnrollmentDelivery(f.t, f.db),
	)
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

func TestEnrollmentDeletion_DeleteChildFromMixedRequestPreservesSharedData(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "mixed")
	request := f.request("mixed", nil)
	target := f.child(request.ID, "Rejected", enrollmentModels.ChildStatusRejected, nil)
	remaining := f.child(request.ID, "Approved", enrollmentModels.ChildStatusApproved, nil)

	change := &enrollmentModels.ChangeRequest{
		RequestID:      request.ID,
		RequestChildID: &target.ID,
		Origin:         enrollmentModels.ChangeRequestOriginParent,
		BaseSnapshot:   map[string]any{}, ProposedSnapshot: map[string]any{}, Diff: map[string]any{},
	}
	require.NoError(t, f.repos.ChangeRequest.Create(f.scope.Context(), change))
	message := &enrollmentModels.ChangeRequestMessage{ChangeRequestID: change.ID, AuthorType: enrollmentModels.ChangeRequestMessageAuthorParent, Body: "remove with child"}
	require.NoError(t, f.repos.ChangeRequestMessage.Create(f.scope.Context(), message))
	guardian := &enrollmentModels.RequestGuardian{RequestID: request.ID, FirstName: "Other", LastName: "Guardian"}
	require.NoError(t, f.repos.RequestGuardian.Create(f.scope.Context(), guardian))
	outbox := enqueueTestEnrollmentEmail(t, db, f.scope.TenantID, request.ID, fmt.Sprintf("mixed-%d", request.ID), map[string]any{})

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
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
	_, err = f.repos.Request.FindByStatusToken(f.scope.Context(), request.StatusToken)
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

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
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
	coGuardian := &enrollmentModels.RequestGuardian{RequestID: request.ID, FirstName: "Preserved", LastName: "Guardian", GuardianProfileID: &profile.ID}
	require.NoError(t, f.repos.RequestGuardian.Create(f.scope.Context(), coGuardian))

	offering := &enrollmentModels.CareOffering{PhaseID: f.phase, Name: "Test care", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}, IsActive: true, CountsAsCare: true, CountsAsCareSet: true, AutoAddGradeLevels: []int{}, SelectionRule: enrollmentModels.SelectionRuleOptional}
	require.NoError(t, f.repos.CareOffering.Create(f.scope.Context(), offering))
	childOffering := &enrollmentModels.RequestChildOffering{RequestChildID: child.ID, CareOfferingID: offering.ID, SelectedDays: []string{"mon"}}
	require.NoError(t, f.repos.RequestChildOffering.Create(f.scope.Context(), childOffering))
	change := &enrollmentModels.ChangeRequest{RequestID: request.ID, RequestChildID: &child.ID, Origin: enrollmentModels.ChangeRequestOriginParent, BaseSnapshot: map[string]any{}, ProposedSnapshot: map[string]any{}, Diff: map[string]any{}}
	require.NoError(t, f.repos.ChangeRequest.Create(f.scope.Context(), change))
	message := &enrollmentModels.ChangeRequestMessage{ChangeRequestID: change.ID, AuthorType: enrollmentModels.ChangeRequestMessageAuthorStaff, AuthorAccountID: &f.actor, Body: "dependent message"}
	require.NoError(t, f.repos.ChangeRequestMessage.Create(f.scope.Context(), message))
	invite := &enrollmentModels.LateInvite{PhaseID: f.phase, TokenHash: fmt.Sprintf("invite-%d", f.scope.TenantID), GuardianEmail: request.GuardianEmail, ExpiresAt: time.Now().Add(time.Hour), CreatedBy: f.actor}
	require.NoError(t, f.repos.LateInvite.Create(f.scope.Context(), invite))
	require.NoError(t, f.repos.LateInvite.MarkUsed(f.scope.Context(), invite.ID, request.ID, time.Now()))
	outbox := enqueueTestEnrollmentEmail(t, db, f.scope.TenantID, request.ID, fmt.Sprintf("request-%d", request.ID), map[string]any{"request_id": request.ID})
	student := testpkg.CreateTestStudentForTenant(t, db, f.scope.TenantID, "Adjustment", "Student", "1a")
	adjustment := &auditModels.EnrollmentOfferingAdjustment{RequestID: request.ID, RequestChildID: child.ID, StudentID: student.ID, ActorAccountID: f.actor, ActorRole: "admin", Reason: "test adjustment", Before: json.RawMessage(`{}`), After: json.RawMessage(`{}`), ChangedAt: time.Now()}
	require.NoError(t, f.repos.EnrollmentOfferingAdjustment.Create(f.scope.Context(), adjustment))

	preview, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
		return f.service(nil, nil).PreviewRequest(ctx, request.ID)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, preview.PreservedGuardianProfiles)
	assert.Equal(t, 1, preview.PreservedParentAccounts)
	assert.Equal(t, 1, preview.UnlinkedGuardianProfiles)
	assert.Equal(t, 1, preview.ParentAccountsWithoutStudents)
	assert.Equal(t, 1, preview.Counts.EmailOutbox)
	assert.Equal(t, 1, preview.Counts.OfferingAdjustments)

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
		return f.service(nil, nil).DeleteRequest(ctx, request.ID, f.actor, "Genehmigte Testanmeldung bereinigen")
	})
	require.NoError(t, err)
	assert.Equal(t, preview.Counts, impact.Counts)
	for _, check := range []struct {
		table string
		id    int64
	}{
		{"enrollment.requests", request.ID},
		{"enrollment.request_children", child.ID},
		{"enrollment.request_child_offerings", childOffering.ID},
		{"enrollment.request_guardians", coGuardian.ID},
		{"enrollment.change_requests", change.ID},
		{"enrollment.change_request_messages", message.ID},
		{"enrollment.late_invites", invite.ID},
		{"audit.enrollment_offering_adjustments", adjustment.ID},
	} {
		assert.Zero(t, tableCount(t, db, check.table, "id = ?", check.id), check.table)
	}
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ? AND status = 'cancelled'", outbox.ID))
	assert.Equal(t, 1, tableCount(t, db, "users.students", "id = ?", student.ID))
	assert.Equal(t, 1, tableCount(t, db, "users.guardian_profiles", "id = ?", profile.ID))
	assert.Equal(t, 1, tableCount(t, db, "auth.accounts", "id = ?", guardianAccount.ID))
	_, err = f.repos.Request.FindByStatusToken(f.scope.Context(), request.StatusToken)
	require.Error(t, err)
	assert.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ?", f.scope.TenantID, request.ID))
}

func TestEnrollmentDeletion_DeleteLastChildAlsoDeletesRequest(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newDeletionTestFixture(t, db, "last-child")
	request := f.request("last-child", nil)
	child := f.child(request.ID, "Withdrawn", enrollmentModels.ChildStatusWithdrawn, nil)

	impact, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
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

	preview, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
		return f.service(nil, nil).PreviewRequest(ctx, request.ID)
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, preview.BlockingStudentIDs)
	_, err = tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
		return f.service(nil, nil).DeleteRequest(ctx, request.ID, f.actor, "Darf nicht gelöscht werden")
	})
	require.ErrorIs(t, err, enrollmentService.ErrEnrollmentDeletionStudentExists)
	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Equal(t, 1, tableCount(t, db, "users.students", "id = ?", student.ID))
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
	_, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
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

	_, err := tenantCall(t, db, tenantA.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
		return tenantA.service(nil, nil).PreviewRequest(ctx, requestB.ID)
	})
	require.ErrorIs(t, err, enrollmentService.ErrEnrollmentDeletionNotFound)
	assert.Equal(t, 1, tableCount(t, db, "enrollment.requests", "id = ?", requestB.ID))
}

type deletionLockSignalRepository struct {
	enrollmentModels.RequestRepository
	targetID int64
	started  chan struct{}
	once     sync.Once
}

func (r *deletionLockSignalRepository) FindByIDForUpdate(ctx context.Context, id int64) (*enrollmentModels.Request, error) {
	if id == r.targetID {
		r.once.Do(func() { close(r.started) })
	}
	return r.RequestRepository.FindByIDForUpdate(ctx, id)
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
	requestRepo := &deletionLockSignalRepository{RequestRepository: f.repos.Request, targetID: request.ID, started: started}
	finished := make(chan error, 1)
	go func() {
		_, deleteErr := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (*enrollmentModels.DeletionImpact, error) {
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

	cleaner := enrollmentService.NewRejectedEnrollmentCleanupService(
		f.repos.Request,
		f.repos.RequestChild,
		f.repos.LateInvite,
		newTestEnrollmentDelivery(t, db),
		cleanupRetentionSettings{days: 90},
		db,
		slog.New(slog.DiscardHandler),
		enrollmentService.RejectedEnrollmentCleanupAuditDependencies{
			Deletion: f.repos.EnrollmentDeletion,
			Audit:    f.repos.EnrollmentDeletionAudit,
		},
	)
	result, err := tenantCall(t, db, f.scope.TenantID, func(ctx context.Context) (enrollmentService.RejectedEnrollmentCleanupResult, error) {
		return cleaner.CleanupRejectedEnrollments(ctx)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.DeletedRequests)
	assert.EqualValues(t, expectedOutboxRows, result.DeletedOutboxRows)
	assert.Zero(t, tableCount(t, db, "enrollment.requests", "id = ?", request.ID))
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ? AND status = 'cancelled'", outbox.ID))
	assert.Equal(t, 1, tableCount(t, db, "audit.enrollment_deletions", "tenant_id = ? AND request_id = ? AND actor_type = 'system' AND actor_account_id IS NULL", f.scope.TenantID, request.ID))
}
