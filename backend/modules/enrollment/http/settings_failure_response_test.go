package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const privateSettingsFailure = "private enrollment settings read failure"

type failedSettingsIntake struct{ RequestService }

func (failedSettingsIntake) GetByStatusToken(context.Context, string) (*Request, []*RequestChild, error) {
	return &Request{ID: 1}, nil, nil
}
func (failedSettingsIntake) EditModeForStatus(context.Context, *Request, []*RequestChild) (string, error) {
	return "", errors.New(privateSettingsFailure)
}
func (failedSettingsIntake) Edit(context.Context, string, EditPatch) error {
	return errors.New(privateSettingsFailure)
}
func (failedSettingsIntake) ReplaceEditable(context.Context, string, SubmitRequest) (*SubmitResult, error) {
	return nil, errors.New(privateSettingsFailure)
}
func (failedSettingsIntake) Submit(context.Context, SubmitRequest) (*SubmitResult, error) {
	return nil, errors.New(privateSettingsFailure)
}

type failedSettingsChange struct{ ChangeRequestService }

func (failedSettingsChange) Create(context.Context, string, CreateChangeRequestInput) (*ChangeRequestAggregate, error) {
	return nil, errors.New(privateSettingsFailure)
}

type settingsFailureSchool struct{ tenantID int64 }

func (s settingsFailureSchool) GetSchoolBySlug(context.Context, string) (*PublicSchool, error) {
	return &PublicSchool{ID: s.tenantID}, nil
}

type settingsFailureCaptcha struct{ capability.CaptchaVerifier }

func (settingsFailureCaptcha) Verify(context.Context, string, string) error { return nil }

func TestEnrollmentResponsesHideSettingsReadFailures(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	rs := &Resource{RequestService: failedSettingsIntake{}, ChangeRequestService: failedSettingsChange{}, CaptchaService: settingsFailureCaptcha{}, SchoolService: settingsFailureSchool{tenantID: testpkg.Tenant(t)}}
	for _, tc := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		{"public status", "GET", "/status/token", rs.getStatus},
		{"edit patch", "PATCH", "/status/token", rs.patchStatus},
		{"edit replacement", "PUT", "/status/token", rs.replaceStatus},
		{"change request create", "POST", "/status/token", rs.createChangeRequest},
		{"submit", "POST", "/school/submit", rs.submitEnrollment},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route := chi.NewRouteContext()
			route.URLParams.Add("statusToken", "token")
			route.URLParams.Add("tenantSlug", "school")
			route.URLParams.Add("slug", "school")
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(testpkg.WithTestTenantRuntime(t, req.Context()), chi.RouteCtxKey, route))
			response := httptest.NewRecorder()
			tc.handler(response, req)
			require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), `"code":"general.server"`)
			assert.NotContains(t, response.Body.String(), privateSettingsFailure)
			assert.NotContains(t, response.Body.String(), `"data"`)
		})
	}
}
