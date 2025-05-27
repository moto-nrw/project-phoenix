package usercontext

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
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
func (req *ProfileUpdateRequest) Bind(r *http.Request) error {
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
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(user, "Current user retrieved successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// getCurrentProfile returns the current user's full profile
func (res *Resource) getCurrentProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(profile, "Current profile retrieved successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// updateCurrentProfile updates the current user's profile
func (res *Resource) updateCurrentProfile(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ProfileUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(profile, "Profile updated successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// getCurrentStaff returns the current user's staff profile
func (res *Resource) getCurrentStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := res.service.GetCurrentStaff(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(staff, "Current staff profile retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getCurrentTeacher returns the current user's teacher profile
func (res *Resource) getCurrentTeacher(w http.ResponseWriter, r *http.Request) {
	teacher, err := res.service.GetCurrentTeacher(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(teacher, "Current teacher profile retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMyGroups returns the educational groups associated with the current user
func (res *Resource) getMyGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Educational groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMyActivityGroups returns the activity groups associated with the current user
func (res *Resource) getMyActivityGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyActivityGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Activity groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMyActiveGroups returns the active groups associated with the current user
func (res *Resource) getMyActiveGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyActiveGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Active groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMySupervisedGroups returns the active groups supervised by the current user
func (res *Resource) getMySupervisedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMySupervisedGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Supervised groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getGroupStudents returns the students in a specific group where the current user has access
func (res *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	students, err := res.service.GetGroupStudents(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(students, "Group students retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getGroupVisits returns the active visits for a specific group where the current user has access
func (res *Resource) getGroupVisits(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	visits, err := res.service.GetGroupVisits(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(visits, "Group visits retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// Avatar upload constants
const (
	maxUploadSize = 5 * 1024 * 1024 // 5MB
	avatarDir     = "public/uploads/avatars"
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
	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	
	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("file too large"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the file from the request
	file, header, err := r.FormFile("avatar")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("no file uploaded"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Check file type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("cannot read file"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	contentType := http.DetectContentType(buffer)
	
	if !allowedImageTypes[contentType] {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid file type. Only JPEG, PNG, and WebP images are allowed"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Reset file reader
	if _, err := file.Seek(0, 0); err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to process file"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user to generate unique filename
	user, err := res.service.GetCurrentUser(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Generate unique filename with user ID
	fileExt := filepath.Ext(header.Filename)
	if fileExt == "" {
		switch contentType {
		case "image/jpeg", "image/jpg":
			fileExt = ".jpg"
		case "image/png":
			fileExt = ".png"
		case "image/webp":
			fileExt = ".webp"
		}
	}
	randomStr, err := generateRandomString(8)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to generate filename"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	filename := fmt.Sprintf("%d_%s%s", user.ID, randomStr, fileExt)
	filePath := filepath.Join(avatarDir, filename)

	// Create avatar directory if it doesn't exist
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to create upload directory"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to save file"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	defer func() {
		if err := dst.Close(); err != nil {
			log.Printf("Error closing destination file: %v", err)
		}
	}()

	// Copy file contents
	if _, err := io.Copy(dst, file); err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to save file"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update user profile with avatar URL
	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filename)
	updatedProfile, err := res.service.UpdateAvatar(r.Context(), avatarURL)
	if err != nil {
		// Clean up uploaded file on error
		if err := os.Remove(filePath); err != nil {
			log.Printf("Error removing uploaded file: %v", err)
		}
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(updatedProfile, "Avatar uploaded successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// deleteAvatar removes the current user's avatar
func (res *Resource) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	// Get current profile to get avatar path
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if avatar exists
	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("no avatar to delete"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete avatar from profile
	updatedProfile, err := res.service.UpdateAvatar(r.Context(), "")
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete file from filesystem
	if strings.HasPrefix(avatarPath, "/uploads/avatars/") {
		filePath := filepath.Join("public", avatarPath)
		if err := os.Remove(filePath); err != nil {
			// Log error but don't fail the request
			log.Printf("Failed to delete avatar file: %v", err)
		}
	}

	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(updatedProfile, "Avatar deleted successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// serveAvatar serves avatar images with authentication
func (res *Resource) serveAvatar(w http.ResponseWriter, r *http.Request) {
	// Get filename from URL
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("filename required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user's profile
	profile, err := res.service.GetCurrentProfile(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if the requested avatar belongs to the current user
	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		render.Status(r, http.StatusNotFound)
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("no avatar found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Extract filename from the user's avatar path
	// Handle both "/uploads/avatars/filename.png" and "filename.png" formats
	userAvatarFilename := filepath.Base(avatarPath)
	
	if userAvatarFilename != filename {
		render.Status(r, http.StatusForbidden)
		if err := render.Render(w, r, common.ErrorForbidden(errors.New("access denied"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Construct the file path
	filePath := filepath.Join(avatarDir, filename)

	// Security check: ensure the path doesn't escape the avatar directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to process path"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	
	// Get absolute path of avatar directory for comparison
	absAvatarDir, err := filepath.Abs(avatarDir)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to process avatar directory"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	
	if !strings.HasPrefix(absPath, absAvatarDir) {
		render.Status(r, http.StatusForbidden)
		if err := render.Render(w, r, common.ErrorForbidden(errors.New("invalid path"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			render.Status(r, http.StatusNotFound)
			if err := render.Render(w, r, common.ErrorNotFound(errors.New("avatar not found"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
		} else {
			render.Status(r, http.StatusInternalServerError)
			if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to read avatar"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
		}
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(errors.New("failed to read avatar info"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Detect content type
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer[:])
	contentType := http.DetectContentType(buffer[:n])
	
	// Reset file position
	if _, err := file.Seek(0, 0); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Cache-Control", "private, max-age=86400") // Cache for 1 day

	// Serve the file
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
