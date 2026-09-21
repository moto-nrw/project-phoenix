package me

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Avatar upload constants
const (
	maxUploadSize   = 5 * 1024 * 1024 // 5MB
	avatarDir       = "public/uploads/avatars"
	globalAvatarDir = "global"
)

// ProfileUpdateRequest represents a profile update request
type ProfileUpdateRequest struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Bio       *string `json:"bio,omitempty"`
}

// Bind validates the profile update request
func (req *ProfileUpdateRequest) Bind(_ *http.Request) error {
	// No required fields for updates - all are optional
	return nil
}

// currentUser is the account the /api/me route renders.
type currentUser struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Email         string     `json:"email"`
	Username      *string    `json:"username,omitempty"`
	Avatar        string     `json:"avatar,omitempty"`
	Active        bool       `json:"active"`
	IsPasswordOTP bool       `json:"is_password_otp"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
}

func currentUserResponse(account identityaccess.AccountMetadata) *currentUser {
	user := currentUser(account)
	return &user
}

// profileResponse renders the profile map the /api/me/profile routes have
// always returned: the account, then the person or the account fallback,
// then the non-empty school-scoped fields.
func profileResponse(profile identityaccess.CallerProfile) map[string]any {
	account := profile.Account
	response := map[string]any{
		"email":      account.Email,
		"username":   account.Username,
		"last_login": account.LastLogin,
	}
	addIfNotEmpty(response, "avatar", account.Avatar)
	if person := profile.Person; person != nil {
		response["id"] = person.ID
		response["first_name"] = person.FirstName
		response["last_name"] = person.LastName
		response["created_at"] = person.CreatedAt
		response["updated_at"] = person.UpdatedAt
		if person.TagID != nil {
			response["rfid_card"] = *person.TagID
		}
	} else {
		response["id"] = account.ID
		response["created_at"] = account.CreatedAt
		response["updated_at"] = account.UpdatedAt
		response["first_name"] = ""
		response["last_name"] = ""
	}
	addIfNotEmpty(response, "bio", profile.Bio)
	addIfNotEmpty(response, "settings", profile.Settings)
	return response
}

func addIfNotEmpty(response map[string]any, key, value string) {
	if value != "" {
		response[key] = value
	}
}

func (res *Resource) getCurrentProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := res.caller.Profile(r.Context())
	res.respond(w, r, profileResponse(profile), err, "Current profile retrieved successfully")
}

func (res *Resource) updateCurrentProfile(w http.ResponseWriter, r *http.Request) {
	req := &ProfileUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if jwt.ClaimsFromCtx(r.Context()).ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errAuthenticationRequired))
		return
	}
	profile, err := res.caller.UpdateProfile(r.Context(), identityaccess.CallerProfileUpdate{
		FirstName: req.FirstName, LastName: req.LastName, Username: req.Username, Bio: req.Bio,
	})
	res.respond(w, r, profileResponse(profile), err, "Profile updated successfully")
}

// uploadAvatar stores an uploaded avatar image and points the caller's
// avatar at it; the file is removed again when the update fails.
func (res *Resource) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	uploaded, err := common.ParseImage(w, r, "avatar", maxUploadSize)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	account, err := res.caller.Account(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	targetDir := filepath.Join(avatarDir, globalAvatarDir)
	filePath, err := common.SaveImage(uploaded.File, targetDir, fmt.Sprintf("%d", account.ID), uploaded.ContentType)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s/%s", globalAvatarDir, filepath.Base(filePath))
	profile, err := res.caller.UpdateAvatar(r.Context(), avatarURL)
	if err != nil {
		common.RemoveImage(filePath)
	}
	res.respond(w, r, profileResponse(profile), err, "Avatar uploaded successfully")
}

// deleteAvatar removes the caller's avatar.
func (res *Resource) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	profile, err := res.caller.Profile(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	if profile.Account.Avatar == "" {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no avatar to delete")))
		return
	}
	updated, err := res.caller.UpdateAvatar(r.Context(), "")
	res.respond(w, r, profileResponse(updated), err, "Avatar deleted successfully")
}

// serveAvatar serves the caller's own avatar image.
func (res *Resource) serveAvatar(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("filename required")))
		return
	}
	profile, err := res.caller.Profile(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	avatarPath := profile.Account.Avatar
	if avatarPath == "" {
		render.Status(r, http.StatusNotFound)
		common.RenderError(w, r, common.ErrorNotFound(errors.New("no avatar found")))
		return
	}
	if filepath.Base(avatarPath) != filename {
		render.Status(r, http.StatusForbidden)
		common.RenderError(w, r, common.ErrorForbidden(errors.New("access denied")))
		return
	}
	filePath, err := common.ResolveStoredPath("public", avatarPath, "/uploads/avatars/")
	if err != nil {
		render.Status(r, http.StatusForbidden)
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}
	common.ServeImage(w, r, filepath.Dir(filePath), filepath.Base(filePath), "private, max-age=86400")
}
