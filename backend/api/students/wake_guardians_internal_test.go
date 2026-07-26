package students

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/stretchr/testify/assert"
)

// TestResourceWakeChildGuardians pins the two guards on the staff-side guardian
// wake helper (#1725): a nil emitter is a silent no-op (tests build a bare
// Resource), and a zero tenant id logs and skips rather than fanning out with no
// tenant context. The happy-path fan-out (emitter present + valid tenant →
// BroadcastChildUpdateToGuardians) is covered end-to-end by the external
// students_test guardian-wake integration test.
func TestResourceWakeChildGuardians(t *testing.T) {
	// nil emitter: no-op, must not panic.
	(&Resource{}).wakeChildGuardians(42, 100)

	// zero tenantID with a present emitter logs and skips (never reaches the
	// emitter). A bare emitter is a non-nil pointer, which is all the guard needs.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	emitter := parentmessaging.NewEmitter(nil, nil, nil, nil, nil, logger)
	(&Resource{ResourceConfig: ResourceConfig{ParentEventEmitter: emitter, Logger: logger}}).
		wakeChildGuardians(0, 100)
	assert.Contains(t, buf.String(), "no tenant context")
}
