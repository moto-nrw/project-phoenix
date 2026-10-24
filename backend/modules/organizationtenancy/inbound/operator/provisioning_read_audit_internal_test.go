package operator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readAuditService struct {
	organizationtenancy.Provisioning
	entries []organizationtenancy.OperatorReadAccess
	err     error
}

func (s *readAuditService) RecordOperatorRead(_ context.Context, entry organizationtenancy.OperatorReadAccess) error {
	s.entries = append(s.entries, entry)
	return s.err
}

func TestOperatorReadAuditPrecedesDisclosure(t *testing.T) {
	t.Parallel()
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "audit unavailable"}[failed], func(t *testing.T) {
			t.Parallel()
			svc := &readAuditService{}
			if failed {
				svc.err = &organizationtenancy.OperatorAuditUnavailableError{Err: errors.New("private diagnostic")}
			}
			resource := NewProvisioningResource(ProvisioningConfig{Service: svc})
			called := false
			router := chi.NewRouter()
			router.Get("/schools/{id}/accounts/{accountId}/mfa", resource.AuditReadAccess(organizationtenancy.ReadSchoolAccountMFA, func(w http.ResponseWriter, _ *http.Request) {
				called = true
				require.Len(t, svc.entries, 1, "audit must precede reading the account")
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/schools/42/accounts/84/mfa?search=untrusted-filter", nil)
			req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 7, Scope: "platform"}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			require.Len(t, svc.entries, 1)
			entry := svc.entries[0]
			assert.EqualValues(t, 7, entry.OperatorID)
			assert.EqualValues(t, 42, entry.SchoolID)
			assert.EqualValues(t, 84, entry.AccountID)
			assert.Equal(t, !failed, called)
			if failed {
				assert.Equal(t, http.StatusServiceUnavailable, response.Code)
				assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
				assert.NotContains(t, response.Body.String(), "private diagnostic")
			} else {
				assert.Equal(t, http.StatusNoContent, response.Code)
			}
		})
	}
}
