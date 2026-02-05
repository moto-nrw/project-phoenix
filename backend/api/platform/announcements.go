package platform

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// AnnouncementsResource handles user-facing announcements endpoints
type AnnouncementsResource struct {
	announcementService platformSvc.AnnouncementService
}

// NewAnnouncementsResource creates a new announcements resource
func NewAnnouncementsResource(announcementService platformSvc.AnnouncementService) *AnnouncementsResource {
	return &AnnouncementsResource{
		announcementService: announcementService,
	}
}

// AnnouncementResponse represents an announcement in the user-facing response
type AnnouncementResponse struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Type        string  `json:"type"`
	Severity    string  `json:"severity"`
	Version     *string `json:"version,omitempty"`
	PublishedAt string  `json:"published_at"`
}

// ErrResponse is an error response struct
type ErrResponse struct {
	HTTPStatusCode int    `json:"-"`
	StatusText     string `json:"status"`
	ErrorText      string `json:"message,omitempty"`
}

// Render implements the render.Renderer interface
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrInvalidRequest creates an error response for invalid requests
func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "error",
		ErrorText:      err.Error(),
	}
}

// ErrInternal creates an internal server error response
func ErrInternal(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "error",
		ErrorText:      message,
	}
}

// GetUnread handles getting unread announcements for the current user
func (rs *AnnouncementsResource) GetUnread(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	announcements, err := rs.announcementService.GetUnreadForUser(r.Context(), userID)
	if err != nil {
		common.RenderError(w, r, ErrInternal("Failed to retrieve announcements"))
		return
	}

	responses := make([]AnnouncementResponse, 0, len(announcements))
	for _, a := range announcements {
		publishedAt := ""
		if a.PublishedAt != nil {
			publishedAt = a.PublishedAt.Format(time.RFC3339)
		}
		responses = append(responses, AnnouncementResponse{
			ID:          a.ID,
			Title:       a.Title,
			Content:     a.Content,
			Type:        a.Type,
			Severity:    a.Severity,
			Version:     a.Version,
			PublishedAt: publishedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Unread announcements retrieved successfully")
}

// MarkSeen handles marking an announcement as seen
func (rs *AnnouncementsResource) MarkSeen(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	announcementID, err := parseID(r, "id")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if err := rs.announcementService.MarkSeen(r.Context(), userID, announcementID); err != nil {
		common.RenderError(w, r, ErrInternal("Failed to mark announcement as seen"))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement marked as seen")
}

// MarkDismissed handles marking an announcement as dismissed
func (rs *AnnouncementsResource) MarkDismissed(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	announcementID, err := parseID(r, "id")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if err := rs.announcementService.MarkDismissed(r.Context(), userID, announcementID); err != nil {
		common.RenderError(w, r, ErrInternal("Failed to mark announcement as dismissed"))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement dismissed")
}

// parseID extracts and validates an ID from the URL
func parseID(r *http.Request, param string) (int64, error) {
	idStr := chi.URLParam(r, param)
	if idStr == "" {
		return 0, errors.New("ID is required")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid ID")
	}
	return id, nil
}
