package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

func TestAuthorizationObserverReceivesStableReason(t *testing.T) {
	t.Parallel()
	var events []securityruntime.AuthorizationEvent
	handler := AuthorizationObserverMiddleware(func(event securityruntime.AuthorizationEvent) {
		events = append(events, event)
	})(authorizationObserverTestHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 1, TenantID: 2}))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if len(events) != 2 || events[0].Outcome != "resolved" || events[1].Reason != "permission_denied" {
		t.Fatalf("authorization events = %+v", events)
	}
}
