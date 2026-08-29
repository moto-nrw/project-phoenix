package usercontext

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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

// Resource handles the user context-related endpoints
type Resource struct {
	service usercontext.UserContextService
	router  chi.Router
	db      *bun.DB
}

// NewResource creates a new user context resource
func NewResource(service usercontext.UserContextService, db *bun.DB) *Resource {
	r := &Resource{
		service: service,
		router:  chi.NewRouter(),
		db:      db,
	}

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Setup routes with proper authentication chain
	r.router.Use(tokenAuth.Verifier())
	r.router.Use(jwt.Authenticator)
	r.router.Use(jwt.TenantMiddleware)
	r.router.Use(common.SecurityPrincipalMiddleware)
	withTx := common.TenantTxMiddleware

	// User profile endpoints
	r.router.With(withTx).Get("/", r.getCurrentUser)
	r.router.With(withTx).Get("/profile", r.getCurrentProfile)
	r.router.With(withTx).Put("/profile", r.updateCurrentProfile)
	r.router.With(withTx).Post("/profile/avatar", r.uploadAvatar)
	r.router.With(withTx).Delete("/profile/avatar", r.deleteAvatar)
	r.router.With(withTx).Get("/profile/avatar/{filename}", r.serveAvatar)
	r.router.With(withTx).Get("/staff", r.getCurrentStaff)
	r.router.With(withTx).Get("/navigation", common.Fetch(func(ctx context.Context) (*usercontext.NavigationContext, error) {
		return r.service.GetNavigationContext(ctx)
	}, ErrorRenderer, "Navigation context retrieved successfully"))
	r.router.With(withTx).Get("/teacher", r.getCurrentTeacher)

	// Group endpoints - authenticated users can access their own groups
	r.router.Route("/groups", func(router chi.Router) {
		// No additional permissions needed - users can always access their own data
		router.With(withTx).Get("/", r.getMyGroups)
		router.With(withTx).Get("/activity", r.getMyActivityGroups)
		router.With(withTx).Get("/active", r.getMyActiveGroups)
		router.With(withTx).Get("/supervised", r.getMySupervisedGroups)

		// Group details (requires group ID)
		router.Route("/{groupID}", func(router chi.Router) {
			router.With(withTx).Get("/students", r.getGroupStudents)
			router.With(withTx).Get("/visits", r.getGroupVisits)
		})
	})

	return r
}

// Router returns the router for this resource
func (r *Resource) Router() chi.Router {
	return r.router
}

// getCurrentUser returns the current authenticated user
func (res *Resource) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, err := res.service.GetCurrentUser(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(user, "Current user retrieved successfully"))
}

// getCurrentProfile returns the current user's full profile
func (res *Resource) getCurrentProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(profile, "Current profile retrieved successfully"))
}

// updateCurrentProfile updates the current user's profile
func (res *Resource) updateCurrentProfile(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ProfileUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert request to map for service
	updates := make(map[string]interface{})
	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}

	// Check for authentication before tenant transaction
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("authentication required")))
		return
	}

	// Update profile
	tenantID := tenant.FromContext(r.Context())
	var profile interface{}
	if err := tenant.WithTenantTx(r.Context(), res.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		profile, txErr = res.service.UpdateCurrentProfile(ctx, updates)
		return txErr
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(profile, "Profile updated successfully"))
}

// getCurrentStaff returns the current user's staff profile
func (res *Resource) getCurrentStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := res.service.GetCurrentStaff(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(staff, "Current staff profile retrieved successfully"))
}

// getCurrentTeacher returns the current user's teacher profile
func (res *Resource) getCurrentTeacher(w http.ResponseWriter, r *http.Request) {
	teacher, err := res.service.GetCurrentTeacher(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(teacher, "Current teacher profile retrieved successfully"))
}

// GroupWithMetadata wraps a group with additional metadata about how the user has access
type GroupWithMetadata struct {
	*education.Group
	ViaSubstitution bool `json:"via_substitution"` // True if access is through a temporary transfer
}

// getMyGroups returns the educational groups associated with the current user
func (res *Resource) getMyGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Best-effort: a substitution-lookup failure should not hide the primary
	// group list. Reads from a nil map yield false, so the response renders
	// with ViaSubstitution=false for every group when this errors.
	substitutedGroupIDs, _ := res.service.GetSubstitutedGroupIDs(r.Context())

	response := make([]GroupWithMetadata, 0, len(groups))
	for _, group := range groups {
		response = append(response, GroupWithMetadata{
			Group:           group,
			ViaSubstitution: substitutedGroupIDs[group.ID],
		})
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(response, "Educational groups retrieved successfully"))
}

// getMyActivityGroups returns the activity groups associated with the current user
func (res *Resource) getMyActivityGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyActivityGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(groups, "Activity groups retrieved successfully"))
}

// getMyActiveGroups returns the active groups associated with the current user
func (res *Resource) getMyActiveGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyActiveGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(groups, "Active groups retrieved successfully"))
}

// getMySupervisedGroups returns the active groups supervised by the current user
func (res *Resource) getMySupervisedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMySupervisedGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(groups, "Supervised groups retrieved successfully"))
}

// getGroupStudents returns the students in a specific group where the current user has access
func (res *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	groupID, err := common.ParseIDParam(r, "groupID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	students, err := res.service.GetGroupStudents(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(students, "Group students retrieved successfully"))
}

// getGroupVisits returns the active visits for a specific group where the current user has access
func (res *Resource) getGroupVisits(w http.ResponseWriter, r *http.Request) {
	groupID, err := common.ParseIDParam(r, "groupID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	visits, err := res.service.GetGroupVisits(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(visits, "Group visits retrieved successfully"))
}

// Avatar upload constants
const (
	maxUploadSize   = 5 * 1024 * 1024 // 5MB
	avatarDir       = "public/uploads/avatars"
	globalAvatarDir = "global"
)

// uploadAvatar handles avatar image upload
func (res *Resource) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	uploaded, err := common.ParseImage(w, r, "avatar", maxUploadSize)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	user, err := res.service.GetCurrentUser(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	targetDir := filepath.Join(avatarDir, globalAvatarDir)
	prefix := fmt.Sprintf("%d", user.ID)
	filePath, err := common.SaveImage(uploaded.File, targetDir, prefix, uploaded.ContentType)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s/%s", globalAvatarDir, filepath.Base(filePath))
	var updatedProfile interface{}
	if err := tenant.WithTenantTx(r.Context(), res.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		updatedProfile, txErr = res.service.UpdateAvatar(ctx, avatarURL)
		return txErr
	}); err != nil {
		common.RemoveImage(filePath)
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(updatedProfile, "Avatar uploaded successfully"))
}

// deleteAvatar removes the current user's avatar
func (res *Resource) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	// Get current profile to get avatar path
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if avatar exists
	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no avatar to delete")))
		return
	}

	// Delete avatar from profile
	tenantID := tenant.FromContext(r.Context())
	var updatedProfile interface{}
	if err := tenant.WithTenantTx(r.Context(), res.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		updatedProfile, txErr = res.service.UpdateAvatar(ctx, "")
		return txErr
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(updatedProfile, "Avatar deleted successfully"))
}

// serveAvatar serves avatar images with authentication
func (res *Resource) serveAvatar(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("filename required")))
		return
	}

	// Validate avatar access for current user
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		render.Status(r, http.StatusNotFound)
		common.RenderError(w, r, common.ErrorNotFound(errors.New("no avatar found")))
		return
	}

	if filepath.Base(avatarPath) != filename {
		render.Status(r, http.StatusForbidden)
		common.RenderError(w, r, common.ErrorForbidden(errors.New("access denied")))
		return
	}

	// Resolve and serve
	filePath, err := common.ResolveStoredPath("public", avatarPath, "/uploads/avatars/")
	if err != nil {
		render.Status(r, http.StatusForbidden)
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	common.ServeImage(w, r, filepath.Dir(filePath), filepath.Base(filePath), "private, max-age=86400")
}
