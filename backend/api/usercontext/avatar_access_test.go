package usercontext

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	usercontextsvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (m *mockAvatarUserContextService) GetCurrentTeacher(context.Context) (*users.Teacher, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAvatarUserContextService) GetMyGroups(context.Context) ([]*education.Group, error) {
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

func TestValidateAvatarAccess_ServiceError(t *testing.T) {
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

	avatarPath, renderer := resource.validateAvatarAccess(requestWithAvatarFilename("avatar.jpg"), "avatar.jpg")

	assert.Empty(t, avatarPath)
	require.NotNil(t, renderer)

	errResp := renderer.(*common.ErrResponse)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
}

func TestValidateAvatarAccess_NoAvatar(t *testing.T) {
	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"email": "test@example.com"}, nil
			},
		},
	}

	avatarPath, renderer := resource.validateAvatarAccess(requestWithAvatarFilename("avatar.jpg"), "avatar.jpg")

	assert.Empty(t, avatarPath)
	require.NotNil(t, renderer)

	errResp := renderer.(*common.ErrResponse)
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestValidateAvatarAccess_FilenameMismatch(t *testing.T) {
	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"avatar": "/uploads/avatars/global/actual.jpg"}, nil
			},
		},
	}

	avatarPath, renderer := resource.validateAvatarAccess(requestWithAvatarFilename("expected.jpg"), "expected.jpg")

	assert.Empty(t, avatarPath)
	require.NotNil(t, renderer)

	errResp := renderer.(*common.ErrResponse)
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
}

func TestValidateAvatarAccess_Success(t *testing.T) {
	resource := &Resource{
		service: &mockAvatarUserContextService{
			getCurrentProfileFunc: func(context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"avatar": "/uploads/avatars/global/expected.jpg"}, nil
			},
		},
	}

	avatarPath, renderer := resource.validateAvatarAccess(requestWithAvatarFilename("expected.jpg"), "expected.jpg")

	assert.Equal(t, "/uploads/avatars/global/expected.jpg", avatarPath)
	assert.Nil(t, renderer)
}

func TestServeAvatar_EmptyFilename(t *testing.T) {
	resource := &Resource{}
	req := requestWithAvatarFilename("")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServeAvatar_InvalidStoredPath(t *testing.T) {
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
