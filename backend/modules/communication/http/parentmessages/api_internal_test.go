package messaging

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
