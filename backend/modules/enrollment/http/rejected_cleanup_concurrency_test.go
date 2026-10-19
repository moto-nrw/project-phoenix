package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	api "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type cleanupLockSignalRepository struct {
	api.EnrollmentDeletionOwner
	targetRequestID int64
	lockStarted     chan struct{}
	once            sync.Once
}

type cleanupRetentionSettings struct {
	days int
}

func (s cleanupRetentionSettings) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}

func (s cleanupRetentionSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, nil
}

func (s cleanupRetentionSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func (s cleanupRetentionSettings) ResolveInt(_ context.Context, key string) (int, error) {
	if key != settingKeyRejectedRetentionDays {
		return 0, fmt.Errorf("unexpected setting key %q", key)
	}
	return s.days, nil
}

func (r *cleanupLockSignalRepository) RequestByID(ctx context.Context, requestID int64, forUpdate bool) (*capability.Request, error) {
	if requestID == r.targetRequestID {
		r.once.Do(func() { close(r.lockStarted) })
	}
	return r.EnrollmentDeletionOwner.RequestByID(ctx, requestID, forUpdate)
}

func TestRejectedEnrollmentCleanup_ConcurrentReopenPreservesRequestAndOutbox(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()
	repos, _, _, _, _, _ := flowRepositories(t, db)

	phase := &capability.Phase{
		Name:             fmt.Sprintf("cleanup-concurrency-%d", scope.TenantID),
		Kind:             capability.PhaseKindSchoolYear,
		ServiceStartDate: capability.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   capability.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true,
	}
	phase.TenantID = scope.TenantID
	require.NoError(t, enrollmentAPI.InsertOwnerPhaseForTest(ctx, repos.Enrollment(), phase))

	request := &enrollmentAPI.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Cleanup",
		GuardianLastName:  "Concurrency",
		GuardianEmail:     fmt.Sprintf("cleanup-%d@example.invalid", scope.TenantID),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  capability.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("cleanup-token-%d", scope.TenantID),
		SubmittedAt:       time.Now(),
	}
	request.TenantID = scope.TenantID
	require.NoError(t, enrollmentAPI.InsertOwnerRequestForTest(ctx, repos.Enrollment(), request))

	oldReview := time.Now().Add(-60 * 24 * time.Hour)
	child := &enrollmentAPI.RequestChild{
		RequestID:      request.ID,
		FirstName:      "Cleanup",
		LastName:       "Child",
		DateOfBirth:    "2018-04-15",
		CustomData:     map[string]any{},
		Status:         capability.ChildStatusRejected,
		ActivationMode: capability.ChildActivationScheduled,
		ReviewedAt:     &oldReview,
	}
	child.TenantID = scope.TenantID
	require.NoError(t, enrollmentAPI.InsertOwnerChildForTest(ctx, repos.Enrollment(), child))

	outboxRow := enqueueTestEnrollmentEmail(t, db, scope.TenantID, request.ID, fmt.Sprintf("reopen-%d", request.ID), map[string]any{"request_id": request.ID})

	// Follow the production edit order: lock the parent first, then reopen the
	// child. Cleanup must wait on that parent instead of taking the child first;
	// otherwise the later cascading request delete creates a lock inversion.
	reopenTx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = reopenTx.Rollback() }()
	var lockedRequestID int64
	require.NoError(t, reopenTx.NewRaw(`
		SELECT id
		FROM enrollment.requests
		WHERE id = ?
		FOR UPDATE
	`, request.ID).Scan(ctx, &lockedRequestID))
	require.Equal(t, request.ID, lockedRequestID)
	_, err = reopenTx.ExecContext(ctx, `
		UPDATE enrollment.request_children
		SET status = ?, reviewed_at = ?
		WHERE id = ?
	`, capability.ChildStatusUnderReview, time.Now(), child.ID)
	require.NoError(t, err)

	lockStarted := make(chan struct{})
	requestRepo := &cleanupLockSignalRepository{
		EnrollmentDeletionOwner: repos.Enrollment(),
		targetRequestID:         request.ID,
		lockStarted:             lockStarted,
	}
	cleaner := api.NewEnrollmentDeletionModule(api.EnrollmentDeletionSources{
		Owner:    requestRepo,
		Delivery: newTestEnrollmentDelivery(t, db),
		Settings: cleanupRetentionSettings{days: 30},
		Logger:   slog.New(slog.DiscardHandler),
	}).Cleanup

	type cleanupOutcome struct {
		result capability.RejectedEnrollmentCleanupResult
		err    error
	}
	finished := make(chan cleanupOutcome, 1)
	go func() {
		result, cleanupErr := cleaner.CleanupRejectedEnrollments(ctx)
		finished <- cleanupOutcome{result: result, err: cleanupErr}
	}()

	select {
	case <-lockStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup never reached the locked child-state recheck")
	}
	require.NoError(t, reopenTx.Commit())

	var outcome cleanupOutcome
	select {
	case outcome = <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not finish after the reopening transaction committed")
	}
	require.NoError(t, outcome.err)
	assert.Zero(t, outcome.result)

	storedRequest, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, repos.Enrollment(), request.ID)
	require.NoError(t, err)
	assert.Equal(t, request.ID, storedRequest.ID)
	storedChild, err := testpkg.TenantQuery1(t, ctx, db, repos.Enrollment().ChildByID, child.ID)
	require.NoError(t, err)
	assert.Equal(t, capability.ChildStatusUnderReview, storedChild.Status)
	assert.Equal(t, 1, tableCount(t, db, "platform.email_outbox", "id = ? AND status = 'pending'", outboxRow.ID))
}

func TestRejectedEnrollmentCleanup_TenantRoleDeletesLateInviteOutboxAndRequest(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	repos, _, _, _, _, _ := flowRepositories(t, db)
	creator := testpkg.CreateTestAccount(t, db, "rejected-cleanup-late-invite")
	var _, requestID, outboxID, linkedInviteID, unrelatedInviteID int64
	defer func() {
		_, _ = db.NewDelete().TableExpr("enrollment.late_invites").Where("tenant_id = ?", scope.TenantID).Exec(context.Background())
	}()
	deliveryPort := newTestEnrollmentDelivery(t, db)

	oldReview := time.Now().Add(-60 * 24 * time.Hour)
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, scope.TenantID, func(ctx context.Context, _ bun.Tx) error {
		phase := &capability.Phase{
			Name:             fmt.Sprintf("cleanup-late-invite-%d", scope.TenantID),
			Kind:             capability.PhaseKindSchoolYear,
			ServiceStartDate: capability.Date(calendar.NewDate(2026, 9, 1)),
			ServiceEndDate:   capability.Date(calendar.NewDate(2027, 7, 31)),
			IsActive:         true,
		}
		if err := enrollmentAPI.InsertOwnerPhaseForTest(ctx, repos.Enrollment(), phase); err != nil {
			return err
		}

		request := &enrollmentAPI.Request{
			PhaseID:           phase.ID,
			GuardianFirstName: "Cleanup",
			GuardianLastName:  "Late Invite",
			GuardianEmail:     fmt.Sprintf("cleanup-late-invite-%d@example.invalid", scope.TenantID),
			ConsentFlags:      map[string]any{},
			CustomData:        map[string]any{},
			SubmissionSource:  capability.RequestSourceLateInvite,
			SourceMetadata:    map[string]any{},
			StatusToken:       fmt.Sprintf("cleanup-late-invite-token-%d", scope.TenantID),
			SubmittedAt:       time.Now(),
		}
		if err := enrollmentAPI.InsertOwnerRequestForTest(ctx, repos.Enrollment(), request); err != nil {
			return err
		}
		requestID = request.ID

		child := &enrollmentAPI.RequestChild{
			RequestID:      request.ID,
			FirstName:      "Cleanup",
			LastName:       "Child",
			DateOfBirth:    "2018-04-15",
			CustomData:     map[string]any{},
			Status:         capability.ChildStatusRejected,
			ActivationMode: capability.ChildActivationScheduled,
			ReviewedAt:     &oldReview,
		}
		if err := enrollmentAPI.InsertOwnerChildForTest(ctx, repos.Enrollment(), child); err != nil {
			return err
		}

		linkedInvite := &capability.LateInvite{
			PhaseID:           phase.ID,
			TokenHash:         fmt.Sprintf("linked-invite-%d", scope.TenantID),
			GuardianEmail:     request.GuardianEmail,
			GuardianFirstName: testpkg.StrPtr("Cleanup"),
			GuardianLastName:  testpkg.StrPtr("Late Invite"),
			ExpiresAt:         time.Now().Add(24 * time.Hour),
			CreatedBy:         creator.ID,
			Reason:            testpkg.StrPtr("Contains enrollment PII"),
		}
		if err := testpkg.TenantCommand1(t, ctx, db, repos.Enrollment().InsertLateInvite, linkedInvite); err != nil {
			return err
		}
		linkedInviteID = linkedInvite.ID
		if err := testpkg.TenantCommand3(t, ctx, db, repos.Enrollment().MarkLateInviteUsed, linkedInvite.ID, request.ID, time.Now()); err != nil {
			return err
		}

		unrelatedInvite := &capability.LateInvite{
			PhaseID:       phase.ID,
			TokenHash:     fmt.Sprintf("unrelated-invite-%d", scope.TenantID),
			GuardianEmail: "unrelated@example.invalid",
			ExpiresAt:     time.Now().Add(24 * time.Hour),
			CreatedBy:     creator.ID,
		}
		if err := testpkg.TenantCommand1(t, ctx, db, repos.Enrollment().InsertLateInvite, unrelatedInvite); err != nil {
			return err
		}
		unrelatedInviteID = unrelatedInvite.ID

		payload, err := json.Marshal(map[string]any{"request_id": request.ID})
		if err != nil {
			return err
		}
		outbox, err := enqueueTestEnrollmentEmailInContext(ctx, deliveryPort, scope.TenantID, request.ID, fmt.Sprintf("tenant-role-%d", request.ID), payload)
		if err != nil {
			return err
		}
		outboxID = outbox.ID
		return nil
	}))

	cleaner := api.NewEnrollmentDeletionModule(api.EnrollmentDeletionSources{
		Owner:    repos.Enrollment(),
		Delivery: deliveryPort,
		Settings: cleanupRetentionSettings{days: 30},
		Logger:   slog.New(slog.DiscardHandler),
	}).Cleanup
	var result capability.RejectedEnrollmentCleanupResult
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, scope.TenantID, func(ctx context.Context, _ bun.Tx) error {
		var cleanupErr error
		result, cleanupErr = cleaner.CleanupRejectedEnrollments(ctx)
		return cleanupErr
	}))
	require.Equal(t, capability.RejectedEnrollmentCleanupResult{
		DeletedRequests:    1,
		DeletedLateInvites: 1,
		DeletedOutboxRows:  1,
	}, result)

	var requestCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.requests WHERE id = ?`, requestID).Scan(context.Background(), &requestCount))
	assert.Zero(t, requestCount)
	var linkedInviteCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.late_invites WHERE id = ?`, linkedInviteID).Scan(context.Background(), &linkedInviteCount))
	assert.Zero(t, linkedInviteCount)
	var outboxCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE id = ? AND status = 'cancelled'`, outboxID).Scan(context.Background(), &outboxCount))
	assert.Equal(t, 1, outboxCount)
	var unrelatedCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.late_invites WHERE id = ?`, unrelatedInviteID).Scan(context.Background(), &unrelatedCount))
	assert.Equal(t, 1, unrelatedCount)
}
