package me

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
)

// fakeAvatarProfiles is a CallerProfiles double whose Profile read is
// injectable; the edits are not reached by the avatar serve path.
type fakeAvatarProfiles struct {
	profileFunc func(ctx context.Context) (identityaccess.CallerProfile, error)
}

func (f *fakeAvatarProfiles) Profile(ctx context.Context) (identityaccess.CallerProfile, error) {
	if f.profileFunc != nil {
		return f.profileFunc(ctx)
	}

	return identityaccess.CallerProfile{}, errors.New("not implemented")
}

func (f *fakeAvatarProfiles) UpdateProfile(context.Context, identityaccess.CallerProfileUpdate) (identityaccess.CallerProfile, error) {
	return identityaccess.CallerProfile{}, errors.New("not implemented")
}

func (f *fakeAvatarProfiles) UpdateAvatar(context.Context, string) (identityaccess.CallerProfile, error) {
	return identityaccess.CallerProfile{}, errors.New("not implemented")
}

func avatarResource(profileFunc func(context.Context) (identityaccess.CallerProfile, error)) *Resource {
	return &Resource{
		caller: identityaccess.CallerContext{
			CallerProfiles: &fakeAvatarProfiles{profileFunc: profileFunc},
		},
	}
}

func profileWithAvatar(avatar string) identityaccess.CallerProfile {
	return identityaccess.CallerProfile{Account: identityaccess.AccountMetadata{Avatar: avatar}}
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

	resource := avatarResource(func(context.Context) (identityaccess.CallerProfile, error) {
		return identityaccess.CallerProfile{}, &identityaccess.CallerError{
			Op:  "get current profile",
			Err: identityaccess.ErrCallerNotAuthenticated,
		}
	})
	req := requestWithAvatarFilename("avatar.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestServeAvatar_NoAvatar(t *testing.T) {
	t.Parallel()

	resource := avatarResource(func(context.Context) (identityaccess.CallerProfile, error) {
		return identityaccess.CallerProfile{Account: identityaccess.AccountMetadata{Email: "test@example.com"}}, nil
	})
	req := requestWithAvatarFilename("avatar.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestServeAvatar_FilenameMismatch(t *testing.T) {
	t.Parallel()

	resource := avatarResource(func(context.Context) (identityaccess.CallerProfile, error) {
		return profileWithAvatar("/uploads/avatars/global/actual.jpg"), nil
	})
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

	resource := avatarResource(func(context.Context) (identityaccess.CallerProfile, error) {
		return profileWithAvatar("/uploads/not-avatars/avatar.jpg"), nil
	})
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
	resource := avatarResource(func(context.Context) (identityaccess.CallerProfile, error) {
		return profileWithAvatar("/uploads/avatars/global/user_abc123.jpg"), nil
	})
	req := requestWithAvatarFilename("user_abc123.jpg")
	rr := httptest.NewRecorder()

	resource.serveAvatar(rr, req)

	// 404 = file not on disk, but all access checks passed (not 400/401/403)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
