// Package common_test pins the exact wire format (HTTP status code + raw
// JSON response bytes) produced by api/iot/common's structured conflict
// renderers.
//
// These are B0 wire-format golden tests for issue #575 (API layer
// technical-debt / error-response consolidation). The upcoming B1/B7
// refactor touches this package's error plumbing. That refactor MUST NOT
// change a single byte of what PyrePortal currently receives on the wire —
// PyrePortal hardcodes these strings and field names for its German UI
// mapping (see the root CLAUDE.md "Key Contracts" section). This file is
// the oracle: it renders through the real render.Render(...) pipeline
// (go-chi/render's json.NewEncoder, which HTML-escapes, emits compact
// JSON, and appends a trailing newline) and asserts the literal body
// string, including the "code" values and the exact "details" key names.
//
// Since issue #1879 PyrePortal reads the activity-capacity details via
// current_occupancy/max_capacity (the keys pinned here), and the capacity
// 409s additionally have per-tenant NoDetails variants (details omitted
// when the checkin.*_capacity_details_enabled setting is off) — pinned by
// the *_NoDetails goldens below.
//
// Changing an expectation in this file requires proof that the wire format
// change is intentional — not just "the refactor made the test fail."
package checkin_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/render"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/stretchr/testify/assert"
)

func renderWireIoTCommon(t *testing.T, renderer render.Renderer) (int, string) {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	err := render.Render(w, r, renderer)
	assert.NoError(t, err)
	return w.Code, w.Body.String()
}

func TestWireFormat_IoTCommon_ErrorRoomCapacityExceeded(t *testing.T) {
	t.Parallel()

	renderer := iotCommon.ErrorRoomCapacityExceeded(42, "Room A", 15, 10)
	gotStatus, gotBody := renderWireIoTCommon(t, renderer)
	assert.Equal(t, 409, gotStatus)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"Room capacity exceeded\",\"code\":\"ROOM_CAPACITY_EXCEEDED\",\"details\":{\"room_id\":42,\"room_name\":\"Room A\",\"current_occupancy\":15,\"max_capacity\":10}}\n",
		gotBody,
	)
}

func TestWireFormat_IoTCommon_ErrorActivityCapacityExceeded(t *testing.T) {
	t.Parallel()

	renderer := iotCommon.ErrorActivityCapacityExceeded(7, "Bastelraum", 5, 4)
	gotStatus, gotBody := renderWireIoTCommon(t, renderer)
	assert.Equal(t, 409, gotStatus)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"Activity capacity exceeded\",\"code\":\"ACTIVITY_CAPACITY_EXCEEDED\",\"details\":{\"activity_id\":7,\"activity_name\":\"Bastelraum\",\"current_occupancy\":5,\"max_capacity\":4}}\n",
		gotBody,
	)
}

func TestWireFormat_IoTCommon_ErrorRoomCapacityExceeded_NoDetails(t *testing.T) {
	t.Parallel()

	// Emitted when checkin.room_capacity_details_enabled is off (issue #1879):
	// same status/code/message, no `details` key at all.
	renderer := iotCommon.ErrorRoomCapacityExceededNoDetails()
	gotStatus, gotBody := renderWireIoTCommon(t, renderer)
	assert.Equal(t, 409, gotStatus)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"Room capacity exceeded\",\"code\":\"ROOM_CAPACITY_EXCEEDED\"}\n",
		gotBody,
	)
}

func TestWireFormat_IoTCommon_ErrorActivityCapacityExceeded_NoDetails(t *testing.T) {
	t.Parallel()

	// Emitted when checkin.activity_capacity_details_enabled is off — the
	// default (issue #1879): same status/code/message, no `details` key.
	renderer := iotCommon.ErrorActivityCapacityExceededNoDetails()
	gotStatus, gotBody := renderWireIoTCommon(t, renderer)
	assert.Equal(t, 409, gotStatus)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"Activity capacity exceeded\",\"code\":\"ACTIVITY_CAPACITY_EXCEEDED\"}\n",
		gotBody,
	)
}

func TestWireFormat_IoTCommon_ErrorStudentAlreadyActive_Degraded(t *testing.T) {
	t.Parallel()

	// All optional fields nil/zero — the lookup-couldn't-resolve-prior-visit
	// path. Only student_id must survive in "details".
	renderer := iotCommon.ErrorStudentAlreadyActive(99, 0, nil, nil, "")
	gotStatus, gotBody := renderWireIoTCommon(t, renderer)
	assert.Equal(t, 409, gotStatus)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"student already has an active visit\",\"code\":\"STUDENT_ALREADY_ACTIVE\",\"details\":{\"student_id\":99}}\n",
		gotBody,
	)
}

func TestWireFormat_IoTCommon_ErrorStudentAlreadyActive_FullyPopulated(t *testing.T) {
	t.Parallel()

	entryTime := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
	roomID := int64(5)
	renderer := iotCommon.ErrorStudentAlreadyActive(99, 123, &entryTime, &roomID, "Room A")
	gotStatus, gotBody := renderWireIoTCommon(t, renderer)
	assert.Equal(t, 409, gotStatus)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"student already has an active visit\",\"code\":\"STUDENT_ALREADY_ACTIVE\",\"details\":{\"student_id\":99,\"existing_visit_id\":123,\"entry_time\":\"2026-07-12T10:30:00Z\",\"room_id\":5,\"room_name\":\"Room A\"}}\n",
		gotBody,
	)
}
