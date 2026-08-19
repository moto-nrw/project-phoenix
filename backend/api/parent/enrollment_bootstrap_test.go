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

// parentBootstrapRequestStub records the per-audience access the handler
// forwarded into the form-load gate. Only the bootstrap loader is exercised;
// every other RequestService method panics through the embedded nil
// interface, which would flag an unintended dependency.
type parentBootstrapRequestStub struct {
	enrollmentService.RequestService
	enrolleeCalled bool
	publicCalled   bool
	access         enrollmentService.EnrolleeAudienceAccess
}

func (s *parentBootstrapRequestStub) LoadEnrolleeFormBootstrap(_ context.Context, _ int64, _ time.Time, _ string, access enrollmentService.EnrolleeAudienceAccess) (*enrollmentService.PublicFormBootstrapData, error) {
	s.enrolleeCalled = true
	s.access = access
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

// A guardian whose link grants parent_portal.enrollment.submit unlocks the
// linked_parents audience. It does NOT unlock existing_students: that needs
// the permission-granting relationship to point at a still-enrolled child,
// which this account does not have (#1663).
func TestGetEnrollmentBootstrap_EligibleGuardianUsesEnrolleeGate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true}
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
	require.True(t, requestSvc.enrolleeCalled)
	assert.True(t, requestSvc.access.LinkedParents,
		"an eligible guardian must unlock the linked_parents audience")
	assert.False(t, requestSvc.access.ExistingStudents,
		"existing_students needs a still-enrolled child, not just any submit permission")
}

// The existing_students audience unlocks only on the enrolled-child fact —
// the same one the picker (ListEnrollable) filters those phases by.
func TestGetEnrollmentBootstrap_EnrolledChildUnlocksExistingStudents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true}
	school.ID = 1
	requestSvc := &parentBootstrapRequestStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:                      true,
			HasGuardianLink:             true,
			HasSubmitPermission:         true,
			HasEnrolledSubmitPermission: true,
		}},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	w := serveBootstrap(t, rs, 7004)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.enrolleeCalled)
	assert.True(t, requestSvc.access.LinkedParents)
	assert.True(t, requestSvc.access.ExistingStudents)
}

// An account without the submit permission (revoked, or applying to a new
// school with no guardian link) gets the zero access value, which is exactly
// the anonymous public gate: open / new_students only, 404 for every
// restricted phase instead of leaking a hidden phase's form (#1663).
func TestGetEnrollmentBootstrap_IneligibleAccountUsesPublicGate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true}
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
	require.True(t, requestSvc.enrolleeCalled)
	assert.Equal(t, enrollmentService.EnrolleeAudienceAccess{}, requestSvc.access,
		"an account without submit permission must unlock no restricted audience")
}

// A guardian who is linked but had the submit permission revoked is treated
// like any non-eligible caller: no restricted audience at all.
func TestGetEnrollmentBootstrap_RevokedPermissionUsesPublicGate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true}
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
	require.True(t, requestSvc.enrolleeCalled)
	assert.Equal(t, enrollmentService.EnrolleeAudienceAccess{}, requestSvc.access,
		"revoked submit permission must unlock no restricted audience")
	assert.False(t, requestSvc.publicCalled)
}

// A hidden school is excluded from the parents-portal picker for everyone
// without a family link there. The direct bootstrap route must apply the same
// rule, or the hidden flag only hides the school from the list while a guessed
// subdomain plus a phase id still serves its form (#1663).
func TestGetEnrollmentBootstrap_HiddenSchoolIsUnreachableWithoutFamilyLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true, Hidden: true}
	school.ID = 1
	requestSvc := &parentBootstrapRequestStub{}
	rs := &Resource{
		// Linked as an account (e.g. staff, or a stale mapping) but with no
		// guardian relationship — exactly the case ListEnrollable refuses.
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked: true,
		}},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	w := serveBootstrap(t, rs, 7005)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, requestSvc.enrolleeCalled, "the form must not load for a hidden school without a family link")
	assert.False(t, requestSvc.publicCalled)
}

// The family-link fact mirrors guard.has_family_link in ListEnrollable: an
// active mapping AND a guardian relationship. An existing family keeps
// reaching its own hidden school's re-enrollment forms.
func TestGetEnrollmentBootstrap_HiddenSchoolLoadsForLinkedFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true, Hidden: true}
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

	w := serveBootstrap(t, rs, 7006)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.enrolleeCalled)
	assert.True(t, requestSvc.access.LinkedParents)
}

// A deactivated school is unreachable for everyone, family link or not — the
// same account-independent gate /auth/tenant/resolve applies.
func TestGetEnrollmentBootstrap_InactiveSchoolIsUnreachable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: false}
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

	w := serveBootstrap(t, rs, 7007)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, requestSvc.enrolleeCalled)
}
