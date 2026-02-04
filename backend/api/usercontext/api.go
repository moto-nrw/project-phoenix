package usercontext

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
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
	service          usercontext.UserContextService
	substitutionRepo education.GroupSubstitutionRepository
	router           chi.Router
}

// NewResource creates a new user context resource
func NewResource(service usercontext.UserContextService, substitutionRepo education.GroupSubstitutionRepository) *Resource {
	r := &Resource{
		service:          service,
		substitutionRepo: substitutionRepo,
		router:           chi.NewRouter(),
	}

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

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

	staff, staffErr := res.service.GetCurrentStaff(r.Context())
	substitutedGroupIDs := res.getSubstitutedGroupIDs(r.Context(), staff, staffErr)

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
func (res *Resource) getSubstitutedGroupIDs(ctx context.Context, staff *users.Staff, staffErr error) map[int64]bool {
	result := make(map[int64]bool)
	if staff == nil || staffErr != nil {
		return result
	}

	today := timezone.TodayUTC()

	activeSubs, err := res.substitutionRepo.FindActiveBySubstitute(ctx, staff.ID, today)
	if err != nil {
		return result
	}

	for _, sub := range activeSubs {
		if sub.RegularStaffID == nil {
			result[sub.GroupID] = true
		}
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

// Avatar upload constants
const (
	maxUploadSize   = 5 * 1024 * 1024 // 5MB
	avatarDir       = "public/uploads/avatars"
	errCloseFileFmt = "Error closing file: %v"
)

// Allowed image types
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
}

// uploadAvatar handles avatar image upload
func (res *Resource) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	file, header, contentType, err := res.parseAndValidateUpload(r)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer closeFile(file)

	user, err := res.service.GetCurrentUser(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	filePath, err := res.saveAvatarFile(file, header, contentType, user.ID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filepath.Base(filePath))
	updatedProfile, err := res.service.UpdateAvatar(r.Context(), avatarURL)
	if err != nil {
		removeFile(filePath)
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(updatedProfile, "Avatar uploaded successfully"))
}

// parseAndValidateUpload validates the multipart upload and returns the file and content type
func (res *Resource) parseAndValidateUpload(r *http.Request) (file io.ReadSeekCloser, header *multipart.FileHeader, contentType string, err error) {
	if r.ParseMultipartForm(maxUploadSize) != nil {
		return nil, nil, "", errors.New("file too large")
	}

	file, header, err = r.FormFile("avatar")
	if err != nil {
		return nil, nil, "", errors.New("no file uploaded")
	}

	contentType, err = detectAndValidateContentType(file)
	if err != nil {
		closeFile(file)
		return nil, nil, "", err
	}

	return file, header, contentType, nil
}

// detectAndValidateContentType reads file header and validates content type
func detectAndValidateContentType(file io.ReadSeeker) (string, error) {
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		return "", errors.New("cannot read file")
	}

	contentType := http.DetectContentType(buffer)
	if !allowedImageTypes[contentType] {
		return "", errors.New("invalid file type. Only JPEG, PNG, and WebP images are allowed")
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", errors.New("failed to process file")
	}

	return contentType, nil
}

// saveAvatarFile saves the uploaded file and returns the file path
func (res *Resource) saveAvatarFile(file io.Reader, header *multipart.FileHeader, contentType string, userID int64) (string, error) {
	fileExt := getFileExtension(header.Filename, contentType)
	randomStr, err := generateRandomString(8)
	if err != nil {
		return "", errors.New("failed to generate filename")
	}

	filename := fmt.Sprintf("%d_%s%s", userID, randomStr, fileExt)
	filePath := filepath.Join(avatarDir, filename)

	if os.MkdirAll(avatarDir, 0755) != nil {
		return "", errors.New("failed to create upload directory")
	}

	dst, err := os.Create(filePath)
	if err != nil {
		return "", errors.New("failed to save file")
	}
	defer closeFileHandle(dst)

	if _, err := io.Copy(dst, file); err != nil {
		return "", errors.New("failed to save file")
	}

	return filePath, nil
}

// getFileExtension returns the file extension, inferring from content type if needed
func getFileExtension(filename, contentType string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return ext
	}

	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

// closeFile safely closes a file
func closeFile(file io.Closer) {
	if err := file.Close(); err != nil {
		slog.Default().Error("file close error", slog.String("error", err.Error()))
	}
}

// closeFileHandle safely closes an os.File
func closeFileHandle(f *os.File) {
	if err := f.Close(); err != nil {
		slog.Default().Error("file close error", slog.String("error", err.Error()))
	}
}

// removeFile attempts to remove a file, logging any error
func removeFile(path string) {
	if err := os.Remove(path); err != nil {
		slog.Default().Error("failed to remove file",
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
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
	updatedProfile, err := res.service.UpdateAvatar(r.Context(), "")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Delete file from filesystem
	if strings.HasPrefix(avatarPath, "/uploads/avatars/") {
		filePath := filepath.Join("public", avatarPath)
		if err := os.Remove(filePath); err != nil {
			// Log error but don't fail the request
			slog.Default().Warn("failed to delete avatar file",
				slog.String("path", filePath),
				slog.String("error", err.Error()))
		}
	}

	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(updatedProfile, "Avatar deleted successfully"))
}

// serveAvatar serves avatar images with authentication
func (res *Resource) serveAvatar(w http.ResponseWriter, r *http.Request) {
	// Get filename from URL
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("filename required")))
		return
	}

	// Validate avatar access for current user
	if err := res.validateAvatarAccess(r, filename); err != nil {
		common.RenderError(w, r, err)
		return
	}

	// Construct and validate file path
	filePath, err := validateAvatarPath(filename)
	if err != nil {
		common.RenderError(w, r, err)
		return
	}

	// Open and serve the file
	res.serveAvatarFile(w, r, filePath, filename)
}

// validateAvatarAccess checks if the current user can access the requested avatar
func (res *Resource) validateAvatarAccess(r *http.Request, filename string) render.Renderer {
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		return ErrorRenderer(err)
	}

	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		render.Status(r, http.StatusNotFound)
		return common.ErrorNotFound(errors.New("no avatar found"))
	}

	if filepath.Base(avatarPath) != filename {
		render.Status(r, http.StatusForbidden)
		return common.ErrorForbidden(errors.New("access denied"))
	}

	return nil
}

// validateAvatarPath validates the file path is within the avatar directory
func validateAvatarPath(filename string) (string, render.Renderer) {
	filePath := filepath.Join(avatarDir, filename)

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", common.ErrorInternalServer(errors.New("failed to process path"))
	}

	absAvatarDir, err := filepath.Abs(avatarDir)
	if err != nil {
		return "", common.ErrorInternalServer(errors.New("failed to process avatar directory"))
	}

	if !strings.HasPrefix(absPath, absAvatarDir) {
		return "", common.ErrorForbidden(errors.New("invalid path"))
	}

	return filePath, nil
}

// serveAvatarFile opens and serves the avatar file
func (res *Resource) serveAvatarFile(w http.ResponseWriter, r *http.Request, filePath, filename string) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			render.Status(r, http.StatusNotFound)
			common.RenderError(w, r, common.ErrorNotFound(errors.New("avatar not found")))
		} else {
			render.Status(r, http.StatusInternalServerError)
			common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to read avatar")))
		}
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Error("file close error", slog.String("error", err.Error()))
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to read avatar info")))
		return
	}

	// Detect content type
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer[:])
	contentType := http.DetectContentType(buffer[:n])

	if _, err := file.Seek(0, 0); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Cache-Control", "private, max-age=86400")

	http.ServeContent(w, r, filename, fileInfo.ModTime(), file)
}

// generateRandomString generates a cryptographically secure random string of specified length
func generateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	// Map random bytes to charset
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b), nil
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
