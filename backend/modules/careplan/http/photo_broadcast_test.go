package careplanhttp

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

func TestResourceBroadcastStudentUpdated(t *testing.T) {
	t.Parallel()

	// nil broadcaster: no-op.
	(&Resource{}).broadcastStudentUpdated(42, 100)

	// zero tenantID logs and skips.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	(testResource(ResourceConfig{
		Broadcaster: testpkg.NewRecordingBroadcaster(), Logger: logger})).broadcastStudentUpdated(0, 100)
	assert.Contains(t, buf.String(), "no tenant context")

	// happy path increments the stub.
	b := testpkg.NewRecordingBroadcaster()
	(testResource(ResourceConfig{
		Broadcaster: b, Logger: slog.Default()})).broadcastStudentUpdated(42, 100)
	assert.Equal(t, 1, len(b.CallsByMethod("tenant")))

	// broadcaster error is logged, never propagated.
	buf.Reset()
	errBroadcaster := testpkg.NewRecordingBroadcaster()
	errBroadcaster.Err = errors.New("hub down")
	(testResource(ResourceConfig{
		Broadcaster: errBroadcaster, Logger: logger})).broadcastStudentUpdated(42, 100)
	assert.Contains(t, buf.String(), "failed to broadcast student update")
}
