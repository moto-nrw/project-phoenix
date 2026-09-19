package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// panickingOperatorAuditLogRepo satisfies the operator audit ledger but
// panics from Create, simulating a driver-level panic in the write path.
type panickingOperatorAuditLogRepo struct {
	operatorAuditWriter
	createCalls atomic.Int32
}

func (r *panickingOperatorAuditLogRepo) Create(_ context.Context, _ *operatorAuditRow) error {
	r.createCalls.Add(1)
	panic("simulated driver panic inside operator audit write")
}

// The operator audit append runs on its own goroutine so the login pipeline
// never pays for the insert. Without the recover guard any panic in that
// path — a nil pointer in the repository, a driver bug, anything — would
// take the whole server process down (#1430 review item #10). The guard
// moved here with the flows (#3331); reaching the end of this test is the
// proof it still holds.
func TestOperatorActionLogRecoversFromAPanickingLedger(t *testing.T) {
	t.Parallel()

	ledger := &panickingOperatorAuditLogRepo{}
	log := operatorActionLog{ledger: ledger, logger: slog.Default()}

	log.RecordOperatorActionAsync(operatorAuditEntry{
		OperatorID: 42, Action: "mfa_trusted_device_added",
		ResourceType: "operator_mfa",
	})

	deadline := time.Now().Add(3 * time.Second)
	for ledger.createCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.Positive(t, ledger.createCalls.Load(),
		"the audit goroutine must have reached Create — the recover guard runs around the panic")

	// Give the recover defer a moment to finish its sentry flush. If the
	// goroutine had crashed the process, this line would never run.
	time.Sleep(100 * time.Millisecond)

	// The seam is still usable after the recovered panic: nothing shared was
	// left corrupted by the unwinding goroutine.
	log.RecordOperatorActionAsync(operatorAuditEntry{
		OperatorID: 42, Action: "mfa_verified",
		ResourceType: "operator_mfa",
	})
	time.Sleep(100 * time.Millisecond)
	assert.GreaterOrEqual(t, ledger.createCalls.Load(), int32(2))
}
