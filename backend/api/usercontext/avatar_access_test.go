package usercontext

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	usercontextsvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/stretchr/testify/assert"
)

type mockAvatarUserContextService struct {
	getCurrentProfileFunc func(ctx context.Context) (map[string]interface{}, error)
}

func (m *mockAvatarUserContextService) GetCurrentUser(context.Context) (*authModel.Account, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetCurrentPerson(context.Context) (*users.Person, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetCurrentStaff(context.Context) (*users.Staff, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) HasCurrentStaff(context.Context) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetNavigationContext(context.Context) (*usercontextsvc.NavigationContext, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetCurrentTeacher(context.Context) (*users.Teacher, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetMyGroups(context.Context) ([]*education.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetMySchoolClasses(context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) ResolveSSESubscription(context.Context) (*usercontextsvc.SSESubscription, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetSubstitutedGroupIDs(context.Context) (map[int64]bool, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetMyActivityGroups(context.Context) ([]*activityModels.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetMyActiveGroups(context.Context) ([]*active.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetMySupervisedGroups(context.Context) ([]*active.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetGroupStudents(context.Context, int64) ([]*users.Student, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetGroupVisits(context.Context, int64) ([]*active.Visit, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetCurrentProfile(ctx context.Context) (map[string]interface{}, error) {
	if m.getCurrentProfileFunc != nil {
		return m.getCurrentProfileFunc(ctx)
	}

	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) UpdateCurrentProfile(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) UpdateAvatar(context.Context, string) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func requestWithAvatarFilename(filename string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/profile/avatar/"+filename, nil)
	routeCtx := chi.NewRouteContext()
	if filename != "" {
		routeCtx.URLParams.Add("filename", filename)
	}

	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func TestServeAvatar_ServiceError(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return nil, &usercontextsvc.UserContextError{
					Op:  "get current profile",
					Err: usercontextsvc.ErrUserNotAuthenticated,
				}
			},
		},
	}
	req := requestWithAvatarFilename("avatar.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestServeAvatar_NoAvatar(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"email": "test@example.com"}, nil
			},
		},
	}
	req := requestWithAvatarFilename("avatar.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestServeAvatar_FilenameMismatch(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"avatar": "/uploads/avatars/global/actual.jpg"}, nil
			},
		},
	}
	req := requestWithAvatarFilename("expected.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestServeAvatar_EmptyFilename(t *testing.T) {
	t.Parallel()

	resource := &Resource{}
	req := requestWithAvatarFilename("")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServeAvatar_InvalidStoredPath(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"avatar": "/uploads/not-avatars/avatar.jpg"}, nil
			},
		},
	}
	req := requestWithAvatarFilename("avatar.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestServeAvatar_MatchingFilename_PassesAccessControl(t *testing.T) {
	t.Parallel()

	// A valid avatar path with matching filename passes all access control guards.
	// The request reaches common.ServeImage which returns 404 because the file
	// doesn't exist on disk — proving the path traversal check, filename match,
	// and stored-path validation all succeeded.
	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"avatar": "/uploads/avatars/global/user_abc123.jpg"}, nil
			},
		},
	}
	req := requestWithAvatarFilename("user_abc123.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	// 404 = file not on disk, but all access checks passed (not 400/401/403)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
