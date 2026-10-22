package owners

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panickingOperatorAuditLogRepo satisfies the operator audit ledger but
// panics from Create, simulating a driver-level panic in the write path.
type panickingOperatorAuditLogRepo struct {
	createCalls atomic.Int32
}

func (r *panickingOperatorAuditLogRepo) Create(_ context.Context, _ *operatorAuditRow) error {
	r.createCalls.Add(1)
	panic("simulated driver panic inside operator audit write")
}

// The operator audit append must never take the server process down: a panic
// in the write path — a nil pointer in the repository, a driver bug, anything —
// is recovered and returned as an error (#1430 review item #10). The guard
// moved here with the flows (#3331).
func TestOperatorActionLogRecoversFromAPanickingLedger(t *testing.T) {
	t.Parallel()

	ledger := &panickingOperatorAuditLogRepo{}
	log := operatorActionLog{ledger: ledger, logger: slog.Default()}

	err := log.RecordOperatorAction(context.Background(), operatorAuditEntry{
		OperatorID: 42, Action: "mfa_trusted_device_added",
		ResourceType: "operator_mfa",
	})
	require.ErrorContains(t, err, "panic in operator mfa audit logging")
	assert.Equal(t, int32(1), ledger.createCalls.Load(), "the recover guard runs around the panic in Create")

	// The seam is still usable after the recovered panic: nothing shared was
	// left corrupted by the unwinding call.
	err = log.RecordOperatorAction(context.Background(), operatorAuditEntry{
		OperatorID: 42, Action: "mfa_verified",
		ResourceType: "operator_mfa",
	})
	require.ErrorContains(t, err, "panic in operator mfa audit logging")
	assert.Equal(t, int32(2), ledger.createCalls.Load())
}
