package students

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
)

type stubTenantBroadcaster struct {
	realtime.Broadcaster
	calls int
	err   error
}

func (s *stubTenantBroadcaster) BroadcastToTenant(_ int64, _ realtime.Event) error {
	s.calls++
	return s.err
}

func TestResourceBroadcastStudentUpdated(t *testing.T) {
	// nil broadcaster: no-op.
	(&Resource{}).broadcastStudentUpdated(42, 100)

	// zero tenantID logs and skips.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	(&Resource{ResourceConfig: ResourceConfig{Broadcaster: &stubTenantBroadcaster{}, Logger: logger}}).broadcastStudentUpdated(0, 100)
	assert.Contains(t, buf.String(), "no tenant context")

	// happy path increments the stub.
	b := &stubTenantBroadcaster{}
	(&Resource{ResourceConfig: ResourceConfig{Broadcaster: b, Logger: slog.Default()}}).broadcastStudentUpdated(42, 100)
	assert.Equal(t, 1, b.calls)

	// broadcaster error is logged, never propagated.
	buf.Reset()
	(&Resource{ResourceConfig: ResourceConfig{Broadcaster: &stubTenantBroadcaster{err: errors.New("hub down")}, Logger: logger}}).broadcastStudentUpdated(42, 100)
	assert.Contains(t, buf.String(), "failed to broadcast student update")
}
