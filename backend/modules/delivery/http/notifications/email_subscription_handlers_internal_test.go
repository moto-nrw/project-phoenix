package notifications

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// emailConsentStub stores the decisions behind the real preference service,
// so the routes are tested with the service's own permission check. Any other
// consent call panics through the nil embedded interface.
type emailConsentStub struct {
	notificationsService.ConsentStore
	stored   map[string]bool
	readFor  []int64
	recorded []emailDecision
}

type emailDecision struct {
	accountID        int64
	notificationType string
	enabled          bool
}

func (s *emailConsentStub) StoredConsent(_ context.Context, accountID int64) (map[string]bool, error) {
	s.readFor = append(s.readFor, accountID)
	return s.stored, nil
}

func (s *emailConsentStub) RecordConsent(_ context.Context, accountID int64, notificationType string, enabled bool) error {
	s.recorded = append(s.recorded, emailDecision{accountID: accountID, notificationType: notificationType, enabled: enabled})
	return nil
}

func emailRouter(db *bun.DB, consent *emailConsentStub) chi.Router {
	return NewResource(nil, nil, notificationsService.NewPreferenceService(consent, nil, nil, nil), db).Router()
}

const enrollmentEmailPath = "/email-subscriptions/" + notificationsService.TypeEnrollmentSubmitted

// The opt-in e-mail "Neue Anmeldung" (#3780) is decided by the caller for the
// caller, and only while holding the type's permission.
func TestEmailSubscriptionRoutes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	manager := testutil.TenantUserTestClaims(73, testpkg.Tenant(t), permissions.ConfigManage)
	teacher := testutil.TeacherTestClaims(74)

	do := func(t *testing.T, router chi.Router, claims jwt.AppClaims, method, path string, body any) (int, string) {
		t.Helper()
		req := testutil.NewAuthenticatedRequest(t, method, path, body, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
		rec := testutil.ExecuteRequest(router, req)
		return rec.Code, rec.Body.String()
	}
	on := map[string]any{"enabled": true}

	t.Run("reads the caller's own decision", func(t *testing.T) {
		prefs := &emailConsentStub{stored: map[string]bool{notificationsService.TypeEnrollmentSubmitted: true}}
		router := emailRouter(db, prefs)

		code, body := do(t, router, manager, http.MethodGet, enrollmentEmailPath, nil)

		require.Equal(t, http.StatusOK, code, body)
		assert.Contains(t, body, `"enabled":true`)
		assert.Equal(t, []int64{73}, prefs.readFor)
	})

	t.Run("records the caller's own decision", func(t *testing.T) {
		prefs := &emailConsentStub{}
		router := emailRouter(db, prefs)

		code, body := do(t, router, manager, http.MethodPut, enrollmentEmailPath, on)

		require.Equal(t, http.StatusNoContent, code, body)
		assert.Equal(t, []emailDecision{{accountID: 73, notificationType: notificationsService.TypeEnrollmentSubmitted, enabled: true}}, prefs.recorded)
	})

	t.Run("without the type's permission nothing is read or written", func(t *testing.T) {
		prefs := &emailConsentStub{}
		router := emailRouter(db, prefs)

		code, _ := do(t, router, teacher, http.MethodGet, enrollmentEmailPath, nil)
		assert.Equal(t, http.StatusForbidden, code)
		code, _ = do(t, router, teacher, http.MethodPut, enrollmentEmailPath, on)
		assert.Equal(t, http.StatusForbidden, code)
		assert.Empty(t, prefs.readFor)
		assert.Empty(t, prefs.recorded)
	})

	t.Run("a push type or a missing decision is rejected", func(t *testing.T) {
		prefs := &emailConsentStub{}
		router := emailRouter(db, prefs)

		code, _ := do(t, router, manager, http.MethodPut, "/email-subscriptions/"+notificationsService.TypePickupUpcoming, on)
		assert.Equal(t, http.StatusNotFound, code, "push types are decided on the profile page")
		code, _ = do(t, router, manager, http.MethodPut, enrollmentEmailPath, map[string]any{})
		assert.Equal(t, http.StatusBadRequest, code, "switching off by omission is refused")
		assert.Empty(t, prefs.recorded)
	})

	t.Run("the school portal does not offer it", func(t *testing.T) {
		prefs := &emailConsentStub{}
		router := NewResource(nil, nil, notificationsService.NewPreferenceService(prefs, nil, nil, nil), db).SchoolRouter()

		code, _ := do(t, router, manager, http.MethodGet, enrollmentEmailPath, nil)
		assert.NotEqual(t, http.StatusOK, code)
		assert.Empty(t, prefs.readFor)
	})
}
