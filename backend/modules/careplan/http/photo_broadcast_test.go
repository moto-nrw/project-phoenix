package careplanhttp_test

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

func TestResourceBroadcastStudentUpdated(t *testing.T) {
	t.Parallel()

	// nil broadcaster: no-op.
	(&careplanhttp.Resource{}).BroadcastStudentUpdatedForTest(42, 100)

	// zero tenantID logs and skips.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	(testResource(careplanhttp.ResourceConfig{
		Broadcaster: testpkg.NewRecordingBroadcaster(), Logger: logger})).BroadcastStudentUpdatedForTest(0, 100)
	assert.Contains(t, buf.String(), "no tenant context")

	// happy path increments the stub.
	b := testpkg.NewRecordingBroadcaster()
	(testResource(careplanhttp.ResourceConfig{
		Broadcaster: b, Logger: slog.Default()})).BroadcastStudentUpdatedForTest(42, 100)
	assert.Equal(t, 1, len(b.CallsByMethod("tenant")))

	// broadcaster error is logged, never propagated.
	buf.Reset()
	errBroadcaster := testpkg.NewRecordingBroadcaster()
	errBroadcaster.Err = errors.New("hub down")
	(testResource(careplanhttp.ResourceConfig{
		Broadcaster: errBroadcaster, Logger: logger})).BroadcastStudentUpdatedForTest(42, 100)
	assert.Contains(t, buf.String(), "failed to broadcast student update")
}
