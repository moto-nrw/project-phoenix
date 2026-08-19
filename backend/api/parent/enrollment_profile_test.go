package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// guardianProfileRepoStub records whether the profile read happened at all.
// Every other repository method panics through the embedded nil interface,
// which would flag an unintended dependency.
type guardianProfileRepoStub struct {
	usersModels.GuardianProfileRepository
	called  bool
	profile *usersModels.GuardianProfileWithChildren
}

func (s *guardianProfileRepoStub) LoadProfileWithChildren(context.Context, int64) (*usersModels.GuardianProfileWithChildren, error) {
	s.called = true
	return s.profile, nil
}

func serveEnrollmentProfile(t *testing.T, rs *Resource, accountID int) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/parent/enrollments/{tenantSlug}/profile", rs.getEnrollmentProfile)
	req := withClaims(
		httptest.NewRequest(http.MethodGet, "/parent/enrollments/testschule/profile", nil),
		accountID,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// A hidden school is excluded from the parents-portal picker for everyone
// without a family link there. The profile route is a direct route too, so it
// must apply the same rule as bootstrap and submit: otherwise a guessed
// subdomain still confirms the school exists and is active, and any stale
// guardian row that outlived a deactivated mapping hands back that family's
// children (#1663).
func TestGetEnrollmentProfile_HiddenSchoolIsUnreachableWithoutFamilyLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true, Hidden: true}
	school.ID = 1
	repo := &guardianProfileRepoStub{}
	rs := &Resource{
		// Linked as an account (e.g. staff, or a stale mapping) but with no
		// guardian relationship — exactly the case ListEnrollable refuses.
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked: true,
		}},
		GuardianProfileLoader: usersService.NewGuardianProfileLoader(repo, db, nil),
		SchoolService:         &parentSubmitSchoolStub{school: school},
		db:                    db,
	}

	w := serveEnrollmentProfile(t, rs, 7101)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, repo.called, "no guardian data may be read for an unreachable school")
}

// The family-link fact mirrors guard.has_family_link in ListEnrollable: an
// active mapping AND a guardian relationship. An existing family keeps
// reaching its own hidden school's profile autofill.
func TestGetEnrollmentProfile_HiddenSchoolLoadsForLinkedFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: true, Hidden: true}
	school.ID = 1
	repo := &guardianProfileRepoStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		GuardianProfileLoader: usersService.NewGuardianProfileLoader(repo, db, nil),
		SchoolService:         &parentSubmitSchoolStub{school: school},
		db:                    db,
	}

	w := serveEnrollmentProfile(t, rs, 7102)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, repo.called, "a linked family must still get its autofill payload")
}

// A deactivated school is unreachable for everyone, family link or not — the
// same account-independent gate /auth/tenant/resolve applies.
func TestGetEnrollmentProfile_InactiveSchoolIsUnreachable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	school := &platformModels.School{Name: "Testschule", Slug: "testschule", Subdomain: "testschule", Active: false}
	school.ID = 1
	repo := &guardianProfileRepoStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		GuardianProfileLoader: usersService.NewGuardianProfileLoader(repo, db, nil),
		SchoolService:         &parentSubmitSchoolStub{school: school},
		db:                    db,
	}

	w := serveEnrollmentProfile(t, rs, 7103)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, repo.called)
}
