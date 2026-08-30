package enrollment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type parentReplyCall struct {
	token           string
	changeRequestID int64
}

// capturingChangeRequestService stays package-local under the shared-double
// rule's channel-capture exception.
type capturingChangeRequestService struct {
	enrollmentService.ChangeRequestService
	calls chan parentReplyCall
}

func newCapturingChangeRequestService() *capturingChangeRequestService {
	return &capturingChangeRequestService{calls: make(chan parentReplyCall, 1)}
}

func (s *capturingChangeRequestService) ParentReply(
	_ context.Context,
	token string,
	changeRequestID int64,
	_ enrollmentService.ChangeRequestMessageInput,
) (*enrollmentService.ChangeRequestAggregate, error) {
	s.calls <- parentReplyCall{token: token, changeRequestID: changeRequestID}
	return &enrollmentService.ChangeRequestAggregate{}, nil
}

func TestReplyToChangeRequestAcceptsPositiveID(t *testing.T) {
	t.Parallel()

	service := newCapturingChangeRequestService()
	resource := &Resource{ChangeRequestService: service}
	recorder := httptest.NewRecorder()

	resource.replyToChangeRequest(recorder, newChangeRequestReplyRequest("status-token", "42"))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	call := requireParentReplyCall(t, service.calls)
	assert.Equal(t, "status-token", call.token)
	assert.Equal(t, int64(42), call.changeRequestID)
}

func TestReplyToChangeRequestRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
	}{
		{name: "missing", id: ""},
		{name: "malformed", id: "not-a-number"},
		{name: "zero", id: "0"},
		{name: "negative", id: "-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newCapturingChangeRequestService()
			resource := &Resource{ChangeRequestService: service}
			recorder := httptest.NewRecorder()

			resource.replyToChangeRequest(recorder, newChangeRequestReplyRequest("status-token", test.id))

			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.JSONEq(t, `{"status":"error","error":"invalid change request"}`, recorder.Body.String())
			assertNoParentReplyCall(t, service.calls)
		})
	}
}

func TestReplyToChangeRequestRejectsMissingTokenIndependently(t *testing.T) {
	t.Parallel()

	service := newCapturingChangeRequestService()
	resource := &Resource{ChangeRequestService: service}
	recorder := httptest.NewRecorder()

	resource.replyToChangeRequest(recorder, newChangeRequestReplyRequest("   ", "42"))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.JSONEq(t, `{"status":"error","error":"invalid change request"}`, recorder.Body.String())
	assertNoParentReplyCall(t, service.calls)
}

func requireParentReplyCall(t *testing.T, calls <-chan parentReplyCall) parentReplyCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	default:
		t.Fatal("service was not called")
		return parentReplyCall{}
	}
}

func assertNoParentReplyCall(t *testing.T, calls <-chan parentReplyCall) {
	t.Helper()
	select {
	case <-calls:
		t.Error("service was called")
	default:
	}
}

func newChangeRequestReplyRequest(statusToken string, changeRequestID string) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		"/requests/status-token/change-requests/42/messages",
		strings.NewReader(`{"body":"Danke"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("statusToken", statusToken)
	routeContext.URLParams.Add("changeRequestId", changeRequestID)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}
