package enrollment_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cleanupLockSignalRepository struct {
	enrollmentModels.RequestRepository
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
	if key != configModel.KeyEnrollmentRejectedRetentionDays {
		return 0, fmt.Errorf("unexpected setting key %q", key)
	}
	return s.days, nil
}

func (r *cleanupLockSignalRepository) FindByIDForUpdate(ctx context.Context, requestID int64) (*enrollmentModels.Request, error) {
	if requestID == r.targetRequestID {
		r.once.Do(func() { close(r.lockStarted) })
	}
	return r.RequestRepository.FindByIDForUpdate(ctx, requestID)
}

func TestRejectedEnrollmentCleanup_ConcurrentReopenPreservesRequestAndOutbox(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()
	repos := repositories.NewFactory(db)
	relatedType := platformModels.EmailRelatedTypeEnrollmentRequest
	var phaseID, requestID int64
	defer func() {
		if requestID > 0 {
			_, _ = repos.EmailOutbox.DeleteByRelatedEntity(ctx, relatedType, requestID)
			testpkg.CleanupTableRecords(t, db, "enrollment.requests", requestID)
		}
		if phaseID > 0 {
			testpkg.CleanupTableRecords(t, db, "enrollment.phases", phaseID)
		}
		testpkg.CleanupTableRecords(t, db, "platform.schools", scope.TenantID)
		testpkg.CleanupTableRecords(t, db, "platform.organizations", scope.TenantID)
	}()

	phase := &enrollmentModels.Phase{
		Name:             fmt.Sprintf("cleanup-concurrency-%d", scope.TenantID),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
		IsActive:         true,
	}
	phase.SetTenantID(scope.TenantID)
	require.NoError(t, repos.Phase.Create(ctx, phase))
	phaseID = phase.ID

	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Cleanup",
		GuardianLastName:  "Concurrency",
		GuardianEmail:     fmt.Sprintf("cleanup-%d@example.invalid", scope.TenantID),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("cleanup-token-%d", scope.TenantID),
		SubmittedAt:       time.Now(),
	}
	request.SetTenantID(scope.TenantID)
	require.NoError(t, repos.Request.Create(ctx, request))
	requestID = request.ID

	oldReview := time.Now().Add(-60 * 24 * time.Hour)
	child := &enrollmentModels.RequestChild{
		RequestID:      request.ID,
		FirstName:      "Cleanup",
		LastName:       "Child",
		DateOfBirth:    timezone.NewDate(2018, 4, 15),
		CustomData:     map[string]any{},
		Status:         enrollmentModels.ChildStatusRejected,
		ActivationMode: enrollmentModels.ChildActivationScheduled,
		ReviewedAt:     &oldReview,
	}
	child.SetTenantID(scope.TenantID)
	require.NoError(t, repos.RequestChild.Create(ctx, child))

	relatedID := request.ID
	outboxRow := &platformModels.EmailOutbox{
		Kind:              platformModels.EmailKindEnrollmentRejected,
		RelatedEntityType: &relatedType,
		RelatedEntityID:   &relatedID,
		Payload:           map[string]any{"request_id": relatedID},
		Status:            platformModels.EmailOutboxStatusPending,
		NextRetryAt:       time.Now(),
	}
	require.NoError(t, repos.EmailOutbox.Create(ctx, outboxRow))

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
	`, enrollmentModels.ChildStatusUnderReview, time.Now(), child.ID)
	require.NoError(t, err)

	lockStarted := make(chan struct{})
	requestRepo := &cleanupLockSignalRepository{
		RequestRepository: repos.Request,
		targetRequestID:   request.ID,
		lockStarted:       lockStarted,
	}
	cleaner := enrollmentService.NewRejectedEnrollmentCleanupService(
		requestRepo,
		repos.RequestChild,
		repos.EmailOutbox,
		cleanupRetentionSettings{days: 30},
		db,
		slog.New(slog.DiscardHandler),
	)

	type cleanupOutcome struct {
		result enrollmentService.RejectedEnrollmentCleanupResult
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

	storedRequest, err := repos.Request.FindByID(ctx, request.ID)
	require.NoError(t, err)
	assert.Equal(t, request.ID, storedRequest.ID)
	storedChild, err := repos.RequestChild.FindByID(ctx, child.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusUnderReview, storedChild.Status)
	_, err = repos.EmailOutbox.FindByID(ctx, outboxRow.ID)
	require.NoError(t, err)
}
