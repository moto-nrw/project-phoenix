package operator

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/stretchr/testify/assert"
)

type auditUnavailableMFA struct{ AccountMFA }

func (auditUnavailableMFA) AccountBelongsToSchool(context.Context, int64, int64) (bool, error) {
	return true, nil
}
func (auditUnavailableMFA) OperatorDisableMFA(context.Context, int64, int64, int64, string) error {
	return identityaccess.ErrOperatorAuditUnavailable
}
func (auditUnavailableMFA) OperatorSetMFAOverride(context.Context, int64, int64, int64, string, string) error {
	return identityaccess.ErrOperatorAuditUnavailable
}
func (auditUnavailableMFA) OperatorSetGlobalMFAOverride(context.Context, int64, int64, string, string) error {
	return identityaccess.ErrOperatorAuditUnavailable
}

func TestOperatorMFAAuditFailureAnswersWithRegisteredCode(t *testing.T) {
	t.Parallel()
	resource := &SchoolAccountMFAResource{TenantMFAService: auditUnavailableMFA{}}
	for name, handler := range map[string]http.HandlerFunc{
		"reset":           resource.ResetSchoolAccountMFA,
		"school override": resource.SetSchoolAccountMFAOverride,
		"global override": resource.SetAccountMFAGlobalOverride,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString(`{"override":"force_off","reason":"test transition"}`))
			req.Header.Set("Content-Type", "application/json")
			req = withOperatorAdminParams(req, "42", "84")
			req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 7, Scope: "platform"}))
			response := httptest.NewRecorder()
			handler(response, req)
			assert.Equal(t, http.StatusServiceUnavailable, response.Code)
			assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
		})
	}
}
