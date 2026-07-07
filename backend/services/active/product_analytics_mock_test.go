package active

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingTracker struct {
	distinctID string
	event      string
	props      map[string]any
	calls      int
}

func (r *recordingTracker) Capture(distinctID, event string, props map[string]any) {
	r.distinctID = distinctID
	r.event = event
	r.props = props
	r.calls++
}

func (r *recordingTracker) Close() error { return nil }

func TestTrackProductEventCapturesTenantScopedEvent(t *testing.T) {
	rec := &recordingTracker{}
	svc := &service{tracker: rec}
	ctx := tenant.WithTenantID(context.Background(), 42)

	svc.trackProductEvent(ctx, "student_checked_in", map[string]any{"method": "rfid"})

	require.Equal(t, 1, rec.calls)
	assert.Equal(t, "school:42", rec.distinctID)
	assert.Equal(t, "student_checked_in", rec.event)
	assert.Equal(t, "rfid", rec.props["method"])
	assert.Equal(t, int64(42), rec.props["school_id"])
	assert.Equal(t, map[string]any{"school": "42"}, rec.props["$groups"])
	// GDPR: events must never carry student identifiers
	assert.NotContains(t, rec.props, "student_id")
}

func TestTrackProductEventSkipsWithoutTenant(t *testing.T) {
	rec := &recordingTracker{}
	svc := &service{tracker: rec}

	svc.trackProductEvent(context.Background(), "student_checked_in", nil)

	assert.Zero(t, rec.calls)
}

func TestTrackProductEventNilTrackerIsSafe(t *testing.T) {
	svc := &service{}
	ctx := tenant.WithTenantID(context.Background(), 42)

	assert.NotPanics(t, func() {
		svc.trackProductEvent(ctx, "room_transfer", nil)
	})
}
