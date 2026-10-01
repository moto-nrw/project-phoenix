package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentMessageWireContractPreservesJSONPayloadAndStringIDs(t *testing.T) {
	t.Parallel()
	refID, appliedBy := int64(12), int64(34)
	createdAt := time.Date(2026, time.September, 4, 8, 15, 0, 0, time.UTC)
	response := toMessageResponses([]communication.ParentMessage{{
		ID: 7, SenderKind: "guardian", SenderName: "Erika", Body: "Ja", CreatedAt: createdAt,
		Kind: "request", EventType: "care", RequestType: "pickup", RequestStatus: "open",
		Payload: json.RawMessage(`{"answer":"Ja"}`), RefTable: "schedule.requests", RefID: &refID,
		AppliedBy: &appliedBy, DecisionReason: "bestätigt", ReadByStaff: true, ReadByGuardian: true,
	}})

	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"id":"7","sender_kind":"guardian","sender_name":"Erika","body":"Ja","created_at":"2026-09-04T08:15:00Z","kind":"request","event_type":"care","request_type":"pickup","request_status":"open","payload":{"answer":"Ja"},"ref_table":"schedule.requests","ref_id":"12","applied_by":"34","decision_reason":"bestätigt","read_by_staff":true,"read_by_guardian":true}]`, string(encoded))
}

func TestParentMessageHTTPErrorContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		status int
	}{
		{communication.ErrParentMessageThreadNotFound, http.StatusNotFound},
		{communication.ErrParentMessagingForbidden, http.StatusForbidden},
		{communication.ErrParentMessagingDisabled, http.StatusForbidden},
		{communication.ErrParentMessageEmptyBody, http.StatusBadRequest},
		{communication.ErrParentMessageBodyTooLong, http.StatusBadRequest},
		{communication.ErrParentMessageInvalidGuardian, http.StatusBadRequest},
		{communication.ErrParentMessageInvalidCountScope, http.StatusBadRequest},
		{communication.ErrParentMessageGuardianAccessRevoked, http.StatusConflict},
		{communication.ErrParentMessageHandledBoundaryRequired, http.StatusConflict},
		{errors.New("database unavailable"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		renderMessagingError(recorder, httptest.NewRequest(http.MethodGet, "/", nil), test.err)
		assert.Equal(t, test.status, recorder.Code)
	}
}

// countScopeService records the scope the handler passes on and answers with
// a fixed one.
type countScopeService struct {
	communication.ParentMessagingCapability
	stored string
	setErr error
}

func (s *countScopeService) ParentMessageCountScope(context.Context) (communication.ParentMessageCountSetting, error) {
	return communication.ParentMessageCountSetting{Scope: s.stored, HasOwnGroups: true}, nil
}

func (s *countScopeService) SetParentMessageCountScope(_ context.Context, scope string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.stored = scope
	return nil
}

// TestCountScopeHandlers pins the wire shape of the personal count scope
// (#3673): {"scope": "..."} in and out, 400 for a malformed body or an
// unknown scope.
func TestCountScopeHandlers(t *testing.T) {
	t.Parallel()

	service := &countScopeService{stored: "all"}
	rs := &Resource{Service: service}

	recorder := httptest.NewRecorder()
	rs.getCountScope(recorder, httptest.NewRequest(http.MethodGet, "/count-scope", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"scope":"all"`)
	assert.Contains(t, recorder.Body.String(), `"has_own_groups":true`)

	recorder = httptest.NewRecorder()
	rs.setCountScope(recorder, httptest.NewRequest(http.MethodPut, "/count-scope", strings.NewReader(`{"scope":"own_groups"}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "own_groups", service.stored)
	assert.Contains(t, recorder.Body.String(), `"scope":"own_groups"`)

	recorder = httptest.NewRecorder()
	rs.setCountScope(recorder, httptest.NewRequest(http.MethodPut, "/count-scope", strings.NewReader(`{`)))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	service.setErr = communication.ErrParentMessageInvalidCountScope
	recorder = httptest.NewRecorder()
	rs.setCountScope(recorder, httptest.NewRequest(http.MethodPut, "/count-scope", strings.NewReader(`{"scope":"mine"}`)))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "own_groups", service.stored, "a rejected scope stores nothing")
}
