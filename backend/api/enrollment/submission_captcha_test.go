package enrollment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type captchaSchoolLookup struct {
	platformService.SchoolService
	school *platformModels.School
}

func (s captchaSchoolLookup) GetSchoolBySlug(context.Context, string) (*platformModels.School, error) {
	return s.school, nil
}

type captchaBlockedSubmission struct {
	enrollmentService.RequestService
}

type requiredCaptchaSettings struct{}

func (requiredCaptchaSettings) HasTenantOverride(context.Context, string) (bool, error) {
	return true, nil
}
func (requiredCaptchaSettings) ResolveBool(context.Context, string) (bool, error) { return true, nil }
func (requiredCaptchaSettings) ResolveString(context.Context, string) (string, error) {
	return "test-only-secret", nil
}

func TestPublicSubmissionRejectsProviderCaptchaBeforeIntake(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("response") != "rejected-token" {
			http.Error(w, "invalid provider request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer provider.Close()
	school := &platformModels.School{}
	school.ID = testpkg.Tenant(t)
	resource := &Resource{
		db: db, SchoolService: captchaSchoolLookup{school: school},
		RequestService: captchaBlockedSubmission{},
		CaptchaService: enrollmentService.NewCaptchaService(enrollmentService.CaptchaServiceConfig{
			Settings: requiredCaptchaSettings{}, VerifyURL: provider.URL,
		}),
	}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/{tenantSlug}/submit", resource.submitEnrollment)
	req := httptest.NewRequest(http.MethodPost, "/school/submit", strings.NewReader(`{"captcha_token":"rejected-token"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	// Any Submit call panics through the nil embedded interface: rejection must precede intake.
	testpkg.TenantRuntimeMiddleware(t, db)(router).ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Equal(t, int32(1), calls.Load())
	require.Contains(t, w.Body.String(), "captcha")
}
