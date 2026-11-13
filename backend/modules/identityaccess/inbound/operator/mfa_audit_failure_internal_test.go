package operator

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

type auditUnavailableMFA struct {
	identityaccess.AccountMFA
	err error
}

func (auditUnavailableMFA) AccountBelongsToSchool(context.Context, int64, int64) (bool, error) {
	return true, nil
}
func (m auditUnavailableMFA) OperatorDisableMFA(context.Context, int64, int64, int64, string) error {
	return m.err
}
func (m auditUnavailableMFA) OperatorSetMFAOverride(context.Context, int64, int64, int64, string, string) error {
	return m.err
}
func (m auditUnavailableMFA) OperatorSetGlobalMFAOverride(context.Context, int64, int64, string, string) error {
	return m.err
}

// Both ledgers answer a refused entry with a retryable 503: the operator's
// own and the school's authentication ledger, whose row commits with the
// reset or override (#6989).
func TestOperatorMFAAuditFailureAnswersWithRegisteredCode(t *testing.T) {
	t.Parallel()
	for failure, want := range map[string]struct {
		err  error
		code string
	}{
		"operator ledger": {identityaccess.ErrOperatorAuditUnavailable, "operator.audit_unavailable"},
		"school ledger":   {errors.Join(identityaccess.ErrMFAStatusUnavailable, errors.New("audit mfa_admin_override: refused")), "identity.mfa_status_unavailable"},
	} {
		resource := &SchoolAccountMFAResource{TenantMFAService: auditUnavailableMFA{err: want.err}}
		for name, handler := range map[string]http.HandlerFunc{
			"reset":           resource.ResetSchoolAccountMFA,
			"school override": resource.SetSchoolAccountMFAOverride,
			"global override": resource.SetAccountMFAGlobalOverride,
		} {
			t.Run(failure+"/"+name, func(t *testing.T) {
				t.Parallel()
				req := httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString(`{"override":"force_off","reason":"test transition"}`))
				req.Header.Set("Content-Type", "application/json")
				req = withOperatorAdminParams(req, "42", "84")
				req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 7, Scope: "platform"}))
				response := httptest.NewRecorder()
				handler(response, req)
				assert.Equal(t, http.StatusServiceUnavailable, response.Code)
				assert.Contains(t, response.Body.String(), `"code":"`+want.code+`"`)
			})
		}
	}
}
