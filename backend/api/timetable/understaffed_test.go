// Handler tests for the deliberately-unstaffed acknowledgement (#1840). Mock
// InstanceService, no DB — mirrors the lifecycle handler tests in
// instances_test.go.
package timetable

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcknowledgeUnderstaffed_SetWithNote(t *testing.T) {
	t.Parallel()

	note := "kein Ersatz verfügbar"
	instance := &scheduleModel.ActivityInstance{
		Status:           scheduleModel.InstanceStatusPlanned,
		UnderstaffedAck:  true,
		UnderstaffedNote: &note,
	}
	instance.ID = int64(7)

	mock := &mockInstanceService{ackRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/7/acknowledge-understaffed", map[string]any{
		"ack": true, "note": note,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Handler forwarded the exact args to the service.
	assert.Equal(t, int64(7), mock.lastAckID)
	assert.True(t, mock.lastAckValue)
	require.NotNil(t, mock.lastAckNote)
	assert.Equal(t, note, *mock.lastAckNote)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(7), data["instance_id"])
	assert.Equal(t, true, data["understaffed_ack"])
	assert.Equal(t, note, data["understaffed_note"])
}

func TestAcknowledgeUnderstaffed_ClearDropsNote(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusPlanned}
	instance.ID = int64(8)

	mock := &mockInstanceService{ackRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/8/acknowledge-understaffed", map[string]any{"ack": false})
	require.Equal(t, http.StatusOK, w.Code)
	assert.False(t, mock.lastAckValue)
	assert.Nil(t, mock.lastAckNote, "empty note normalizes to nil")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, false, data["understaffed_ack"])
	_, hasNote := data["understaffed_note"]
	assert.False(t, hasNote, "cleared note is omitted from the response")
}

func TestAcknowledgeUnderstaffed_EmptyNoteNormalized(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusPlanned, UnderstaffedAck: true}
	instance.ID = int64(9)
	mock := &mockInstanceService{ackRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/9/acknowledge-understaffed", map[string]any{"ack": true, "note": ""})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, mock.lastAckNote, "blank note must not be persisted")
}

func TestAcknowledgeUnderstaffed_WhitespaceNoteNormalized(t *testing.T) {
	t.Parallel()

	// A whitespace-only note must normalize to nil, not persist as an
	// empty-looking reason — matching trimReason on the substitute path.
	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusPlanned, UnderstaffedAck: true}
	instance.ID = int64(10)
	mock := &mockInstanceService{ackRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/10/acknowledge-understaffed", map[string]any{"ack": true, "note": "   "})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, mock.lastAckNote, "whitespace-only note must not be persisted")
}

func TestAcknowledgeUnderstaffed_NoteTrimmed(t *testing.T) {
	t.Parallel()

	// Surrounding whitespace is stripped before persisting so the stored reason
	// is exactly the trimmed text.
	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusPlanned, UnderstaffedAck: true}
	instance.ID = int64(11)
	mock := &mockInstanceService{ackRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/11/acknowledge-understaffed", map[string]any{"ack": true, "note": "  kein Ersatz  "})
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.lastAckNote)
	assert.Equal(t, "kein Ersatz", *mock.lastAckNote, "note is trimmed before persisting")
}

func TestAcknowledgeUnderstaffed_NoteTooLong(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/1/acknowledge-understaffed", map[string]any{
		"ack": true, "note": strings.Repeat("x", understaffedAckNoteMaxLength+1),
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAcknowledgeUnderstaffed_InvalidID(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/nope/acknowledge-understaffed", map[string]any{"ack": true})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAcknowledgeUnderstaffed_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/1/acknowledge-understaffed", map[string]any{"ack": true})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAcknowledgeUnderstaffed_CompletedRejected(t *testing.T) {
	t.Parallel()

	// Service refuses a terminal-status instance → 409 via the shared lifecycle
	// error mapper.
	mock := &mockInstanceService{ackErr: scheduleSvc.ErrInvalidInstanceTransition}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/5/acknowledge-understaffed", map[string]any{"ack": true})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAcknowledgeUnderstaffed_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{ackErr: scheduleSvc.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)

	w := doPost(t, router, "/instances/5/acknowledge-understaffed", map[string]any{"ack": true})
	assert.Equal(t, http.StatusNotFound, w.Code)
}
