package platform_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// panickingOperatorAuditLogRepo satisfies OperatorAuditLogRepository but
// panics from Create. Used to simulate a driver-level panic in the
// audit-write path so the test can verify the goroutine's recover guard
// keeps the process alive.
type panickingOperatorAuditLogRepo struct {
	platformModels.OperatorAuditLogRepository
	createCalls atomic.Int32
}

func (r *panickingOperatorAuditLogRepo) Create(_ context.Context, _ *platformModels.OperatorAuditLog) error {
	r.createCalls.Add(1)
	panic("simulated driver panic inside operator audit write")
}

// TestOperatorMFAService_RecordAudit_PanicInGoroutineRecovers exercises
// Item #10 from the PR #1430 review: the operator audit goroutine had
// no defer/recover, so any panic in the write path (nil-pointer in
// repo, bun driver bug, json marshal misuse) would crash the entire
// server process. The fix mirrors mfaService.recordAuthEvent's
// recovery pattern — we panic in a fake repo and confirm the goroutine
// dies quietly without taking the test runner with it.
func TestOperatorMFAService_RecordAudit_PanicInGoroutineRecovers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, repos, db := newTestOperatorMFAService(t)

	stub := &panickingOperatorAuditLogRepo{
		OperatorAuditLogRepository: repos.OperatorAuditLog,
	}
	repos.OperatorAuditLog = stub

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: operatorMFATestJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	// Issuing a trusted device triggers a recordAudit call with
	// ActionMFATrustedDeviceAdded — exactly the path that races into
	// the goroutine we want to test. Without the recover guard the
	// panic would crash the test binary, so reaching the next line is
	// itself the proof the fix works.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, err := svc.IssueTrustedDevice(ctx, op.ID,
			"ua-panic", net.ParseIP("203.0.113.99"))
		// IssueTrustedDevice returns before the audit goroutine runs
		// — the panic happens async, so the outer call sees no error.
		assert.NoError(t, err)
	}()
	wg.Wait()

	// Wait for the goroutine to fire (audit is fire-and-forget). The
	// panicking stub increments a counter so we can poll deterministically.
	deadline := time.Now().Add(3 * time.Second)
	for stub.createCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.Positive(t, stub.createCalls.Load(),
		"audit goroutine must have invoked Create — the recover guard runs around the panic")

	// Give the recover-defer a moment to finish its sentry flush.
	// If the goroutine had crashed the process, this line would never
	// execute. Reaching it is the assertion.
	time.Sleep(100 * time.Millisecond)

	// Sanity: the service itself is still usable post-panic — no
	// shared state corrupted by the unwinding goroutine.
	enrolled, err := svc.HasEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.True(t, enrolled,
		"service must remain functional after a recovered audit-goroutine panic")
}
