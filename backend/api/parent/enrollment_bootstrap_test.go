package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// parentBootstrapRequestStub records which form-load gate the handler chose.
// Only the two bootstrap loaders are exercised; every other RequestService
// method panics through the embedded nil interface, which would flag an
// unintended dependency.
type parentBootstrapRequestStub struct {
	enrollmentService.RequestService
	enrolleeCalled bool
	publicCalled   bool
}

func (s *parentBootstrapRequestStub) LoadEnrolleeFormBootstrap(context.Context, int64, time.Time, string) (*enrollmentService.PublicFormBootstrapData, error) {
	s.enrolleeCalled = true
	return &enrollmentService.PublicFormBootstrapData{Phase: &enrollmentModels.Phase{}}, nil
}

func (s *parentBootstrapRequestStub) LoadPublicFormBootstrap(context.Context, int64, time.Time, string) (*enrollmentService.PublicFormBootstrapData, error) {
	s.publicCalled = true
	return &enrollmentService.PublicFormBootstrapData{Phase: &enrollmentModels.Phase{}}, nil
}

func serveBootstrap(t *testing.T, rs *Resource, accountID int) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/parent/enrollments/{tenantSlug}/bootstrap/{phaseId}", rs.getEnrollmentBootstrap)
	req := withClaims(
		httptest.NewRequest(http.MethodGet, "/parent/enrollments/testschule/bootstrap/4242", nil),
		accountID,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// A guardian whose link grants parent_portal.enrollment.submit gets the
// enrollee gate, which serves audience-restricted (linked_parents) phases.
func TestGetEnrollmentBootstrap_EligibleGuardianUsesEnrolleeGate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	school := &platformModels.School{Name: "Testschule", Slug: "testschule"}
	school.ID = 1
	requestSvc := &parentBootstrapRequestStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	w := serveBootstrap(t, rs, 7001)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, requestSvc.enrolleeCalled, "an eligible guardian must reach the enrollee gate")
	assert.False(t, requestSvc.publicCalled)
}

// An account without the submit permission (revoked, or applying to a new
// school with no guardian link) falls back to the public gate, which 404s
// linked_parents phases instead of leaking a hidden phase's form (#1663).
func TestGetEnrollmentBootstrap_IneligibleAccountUsesPublicGate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	school := &platformModels.School{Name: "Testschule", Slug: "testschule"}
	school.ID = 1
	requestSvc := &parentBootstrapRequestStub{}
	rs := &Resource{
		// Zero-value status: no guardian link, no submit permission.
		ParentService:  &fakeParentService{},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	w := serveBootstrap(t, rs, 7002)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, requestSvc.publicCalled, "an account without submit permission must use the public gate")
	assert.False(t, requestSvc.enrolleeCalled)
}

// A guardian who is linked but had the submit permission revoked is treated
// like any non-eligible caller: the public gate, never the enrollee gate.
func TestGetEnrollmentBootstrap_RevokedPermissionUsesPublicGate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	school := &platformModels.School{Name: "Testschule", Slug: "testschule"}
	school.ID = 1
	requestSvc := &parentBootstrapRequestStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: false,
		}},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	w := serveBootstrap(t, rs, 7003)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, requestSvc.publicCalled, "revoked submit permission must not reach the enrollee gate")
	assert.False(t, requestSvc.enrolleeCalled)
}
