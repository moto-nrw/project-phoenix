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

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// fakeEnrollmentForms records what the portal hands Enrollment's form flow:
// the per-audience access of a form load and the school, account,
// eligibility and client IP of a submission. The owner side of the flow
// (wire shape, stamping, rendering) is pinned by the enrollment routes'
// ParentForms tests.
type fakeEnrollmentForms struct {
	loadCalled       bool
	linkedParents    bool
	existingStudents bool
	submitCalled     bool
	schoolID         int64
	accountID        int64
	submitEligible   bool
	clientIP         string
}

func (f *fakeEnrollmentForms) LoadFormBootstrap(_ context.Context, _ int64, _ time.Time, _ string, linkedParents, existingStudents bool) (EnrollmentResponse, error) {
	f.loadCalled = true
	f.linkedParents = linkedParents
	f.existingStudents = existingStudents
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }, nil
}

func (*fakeEnrollmentForms) RenderFormBootstrapError(w http.ResponseWriter, _ *http.Request, _ error) {
	w.WriteHeader(http.StatusInternalServerError)
}

func (f *fakeEnrollmentForms) DecodeSubmission(*http.Request) (EnrollmentSubmit, error) {
	return func(_ context.Context, schoolID, accountID int64, submitEligible bool, clientIP string) (EnrollmentResponse, error) {
		f.submitCalled = true
		f.schoolID, f.accountID, f.submitEligible, f.clientIP = schoolID, accountID, submitEligible, clientIP
		return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }, nil
	}, nil
}

func (*fakeEnrollmentForms) RenderSubmitError(w http.ResponseWriter, _ *http.Request, _ error) {
	w.WriteHeader(http.StatusBadRequest)
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

	school := &EnrollmentSchool{Active: true}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7001)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.loadCalled)
	assert.True(t, requestSvc.linkedParents,
		"an eligible guardian must unlock the linked_parents audience")
	assert.False(t, requestSvc.existingStudents,
		"existing_students needs a still-enrolled child, not just any submit permission")
}

// The existing_students audience unlocks only on the enrolled-child fact —
// the same one the picker (ListEnrollable) filters those phases by.
func TestGetEnrollmentBootstrap_EnrolledChildUnlocksExistingStudents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &EnrollmentSchool{Active: true}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:                      true,
			HasGuardianLink:             true,
			HasSubmitPermission:         true,
			HasEnrolledSubmitPermission: true,
		}},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7004)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.loadCalled)
	assert.True(t, requestSvc.linkedParents)
	assert.True(t, requestSvc.existingStudents)
}

// An account without the submit permission (revoked, or applying to a new
// school with no guardian link) gets the zero access value, which is exactly
// the anonymous public gate: open / new_students only, 404 for every
// restricted phase instead of leaking a hidden phase's form (#1663).
func TestGetEnrollmentBootstrap_IneligibleAccountUsesPublicGate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &EnrollmentSchool{Active: true}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		// Zero-value status: no guardian link, no submit permission.
		ParentService:   &fakeParentService{},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7002)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.loadCalled)
	assert.False(t, requestSvc.linkedParents || requestSvc.existingStudents,
		"an account without submit permission must unlock no restricted audience")
}

// A guardian who is linked but had the submit permission revoked is treated
// like any non-eligible caller: no restricted audience at all.
func TestGetEnrollmentBootstrap_RevokedPermissionUsesPublicGate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &EnrollmentSchool{Active: true}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: false,
		}},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7003)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.loadCalled)
	assert.False(t, requestSvc.linkedParents || requestSvc.existingStudents,
		"revoked submit permission must unlock no restricted audience")
}

// A hidden school is excluded from the parents-portal picker for everyone
// without a family link there. The direct bootstrap route must apply the same
// rule, or the hidden flag only hides the school from the list while a guessed
// subdomain plus a phase id still serves its form (#1663).
func TestGetEnrollmentBootstrap_HiddenSchoolIsUnreachableWithoutFamilyLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &EnrollmentSchool{Active: true, Hidden: true}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		// Linked as an account (e.g. staff, or a stale mapping) but with no
		// guardian relationship — exactly the case ListEnrollable refuses.
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked: true,
		}},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7005)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, requestSvc.loadCalled, "the form must not load for a hidden school without a family link")
}

// The family-link fact mirrors guard.has_family_link in ListEnrollable: an
// active mapping AND a guardian relationship. An existing family keeps
// reaching its own hidden school's re-enrollment forms.
func TestGetEnrollmentBootstrap_HiddenSchoolLoadsForLinkedFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &EnrollmentSchool{Active: true, Hidden: true}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7006)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, requestSvc.loadCalled)
	assert.True(t, requestSvc.linkedParents)
}

// A deactivated school is unreachable for everyone, family link or not — the
// same account-independent gate /auth/tenant/resolve applies.
func TestGetEnrollmentBootstrap_InactiveSchoolIsUnreachable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &EnrollmentSchool{Active: false}
	school.ID = 1
	requestSvc := &fakeEnrollmentForms{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		EnrollmentForms: requestSvc,
		SchoolService:   &parentSubmitSchoolStub{school: school},
		db:              db,
	}

	w := serveBootstrap(t, rs, 7007)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, requestSvc.loadCalled)
}
