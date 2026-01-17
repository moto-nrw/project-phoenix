package usercontext

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
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
}

// NewResource creates a new user context resource
func NewResource(service usercontext.UserContextService) *Resource {
	r := &Resource{
		service: service,
		router:  chi.NewRouter(),
	}

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Setup routes with proper authentication chain
	r.router.Use(tokenAuth.Verifier())
	r.router.Use(jwt.Authenticator)

	// User profile endpoints
	r.router.Get("/", r.getCurrentUser)
	r.router.Get("/profile", r.getCurrentProfile)
	r.router.Put("/profile", r.updateCurrentProfile)
	r.router.Post("/profile/avatar", r.uploadAvatar)
	r.router.Delete("/profile/avatar", r.deleteAvatar)
	r.router.Get("/profile/avatar/{filename}", r.serveAvatar)
	r.router.Get("/staff", r.getCurrentStaff)
	r.router.Get("/teacher", r.getCurrentTeacher)

	// Group endpoints - authenticated users can access their own groups
	r.router.Route("/groups", func(router chi.Router) {
		// No additional permissions needed - users can always access their own data
		router.Get("/", r.getMyGroups)
		router.Get("/activity", r.getMyActivityGroups)
		router.Get("/active", r.getMyActiveGroups)
		router.Get("/supervised", r.getMySupervisedGroups)

		// Group details (requires group ID)
		router.Route("/{groupID}", func(router chi.Router) {
			router.Get("/students", r.getGroupStudents)
			router.Get("/visits", r.getGroupVisits)
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

	// Update profile
	profile, err := res.service.UpdateCurrentProfile(r.Context(), updates)
	if err != nil {
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

	// Get substituted group IDs if user is a staff member
	var substitutedGroupIDs map[int64]bool
	if staff, staffErr := res.service.GetCurrentStaff(r.Context()); staffErr == nil && staff != nil {
		substitutedGroupIDs = res.getSubstitutedGroupIDs(r.Context(), staff.ID)
	} else {
		substitutedGroupIDs = make(map[int64]bool)
	}

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

// getSubstitutedGroupIDs returns a map of group IDs that the user has access to via substitution
func (res *Resource) getSubstitutedGroupIDs(ctx context.Context, staffID int64) map[int64]bool {
	result, err := res.service.GetActiveSubstitutionGroupIDs(ctx, staffID)
	if err != nil {
		return make(map[int64]bool)
	}
	return result
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

// uploadAvatar handles avatar image upload
func (res *Resource) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, usercontext.MaxAvatarSize)

	if r.ParseMultipartForm(usercontext.MaxAvatarSize) != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("file too large")))
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no file uploaded")))
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithError(err).Warn("failed to close uploaded avatar file")
			}
		}
	}()

	input := usercontext.AvatarUploadInput{
		File:     file,
		Filename: header.Filename,
	}

	updatedProfile, err := res.service.UploadAvatar(r.Context(), input)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(updatedProfile, "Avatar uploaded successfully"))
}

// deleteAvatar removes the current user's avatar
func (res *Resource) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	updatedProfile, err := res.service.DeleteAvatar(r.Context())
	if err != nil {
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

	if _, errRenderer := validateAvatarPath(filename); errRenderer != nil {
		common.RenderError(w, r, errRenderer)
		return
	}

	if err := res.service.ValidateAvatarAccess(r.Context(), filename); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	storedFile, err := res.service.GetAvatarFile(r.Context(), filename)
	if err != nil {
		if errors.Is(err, port.ErrFileNotFound) {
			render.Status(r, http.StatusNotFound)
			common.RenderError(w, r, common.ErrorNotFound(errors.New("avatar not found")))
			return
		}
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to load avatar")))
		return
	}

	res.serveAvatarFile(w, r, storedFile, filename)
}

// serveAvatarFile opens and serves the avatar file
func (res *Resource) serveAvatarFile(w http.ResponseWriter, r *http.Request, storedFile port.StoredFile, filename string) {
	defer func() {
		if err := storedFile.Reader.Close(); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithError(err).Warn("failed to close avatar file")
			}
		}
	}()

	contentType := storedFile.ContentType
	reader := storedFile.Reader

	if seeker, ok := reader.(io.ReadSeeker); ok {
		if contentType == "" {
			buffer := make([]byte, 512)
			n, _ := seeker.Read(buffer)
			contentType = http.DetectContentType(buffer[:n])
			if _, err := seeker.Seek(0, 0); err != nil {
				render.Status(r, http.StatusInternalServerError)
				common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to read avatar")))
				return
			}
		}

		w.Header().Set("Content-Type", contentType)
		if storedFile.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(storedFile.Size, 10))
		}
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeContent(w, r, filename, storedFile.ModTime, seeker)
		return
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to read avatar")))
		return
	}

	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, filename, storedFile.ModTime, bytes.NewReader(data))
}

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// GetCurrentUserHandler returns the getCurrentUser handler
func (r *Resource) GetCurrentUserHandler() http.HandlerFunc { return r.getCurrentUser }

// GetCurrentProfileHandler returns the getCurrentProfile handler
func (r *Resource) GetCurrentProfileHandler() http.HandlerFunc { return r.getCurrentProfile }

// UpdateCurrentProfileHandler returns the updateCurrentProfile handler
func (r *Resource) UpdateCurrentProfileHandler() http.HandlerFunc { return r.updateCurrentProfile }

// UploadAvatarHandler returns the uploadAvatar handler
func (r *Resource) UploadAvatarHandler() http.HandlerFunc { return r.uploadAvatar }

// DeleteAvatarHandler returns the deleteAvatar handler
func (r *Resource) DeleteAvatarHandler() http.HandlerFunc { return r.deleteAvatar }

// ServeAvatarHandler returns the serveAvatar handler
func (r *Resource) ServeAvatarHandler() http.HandlerFunc { return r.serveAvatar }

// GetCurrentStaffHandler returns the getCurrentStaff handler
func (r *Resource) GetCurrentStaffHandler() http.HandlerFunc { return r.getCurrentStaff }

// GetCurrentTeacherHandler returns the getCurrentTeacher handler
func (r *Resource) GetCurrentTeacherHandler() http.HandlerFunc { return r.getCurrentTeacher }

// GetMyGroupsHandler returns the getMyGroups handler
func (r *Resource) GetMyGroupsHandler() http.HandlerFunc { return r.getMyGroups }

// GetMyActivityGroupsHandler returns the getMyActivityGroups handler
func (r *Resource) GetMyActivityGroupsHandler() http.HandlerFunc { return r.getMyActivityGroups }

// GetMyActiveGroupsHandler returns the getMyActiveGroups handler
func (r *Resource) GetMyActiveGroupsHandler() http.HandlerFunc { return r.getMyActiveGroups }

// GetMySupervisedGroupsHandler returns the getMySupervisedGroups handler
func (r *Resource) GetMySupervisedGroupsHandler() http.HandlerFunc { return r.getMySupervisedGroups }

// GetGroupStudentsHandler returns the getGroupStudents handler
func (r *Resource) GetGroupStudentsHandler() http.HandlerFunc { return r.getGroupStudents }

// GetGroupVisitsHandler returns the getGroupVisits handler
func (r *Resource) GetGroupVisitsHandler() http.HandlerFunc { return r.getGroupVisits }
