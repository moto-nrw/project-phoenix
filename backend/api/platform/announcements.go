package platform

import (
	"errors"
	"net/http"
	"time"

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

// GetUnread handles getting unread announcements for the current user
func (rs *AnnouncementsResource) GetUnread(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	// Get the user's primary role (first role in the array)
	userRole := ""
	if len(claims.Roles) > 0 {
		userRole = claims.Roles[0]
	}

	announcements, err := rs.announcementService.GetUnreadForUser(r.Context(), userID, userRole)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to retrieve announcements")))
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

// GetUnreadCount handles getting the count of unread announcements
func (rs *AnnouncementsResource) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	// Get the user's primary role
	userRole := ""
	if len(claims.Roles) > 0 {
		userRole = claims.Roles[0]
	}

	count, err := rs.announcementService.CountUnread(r.Context(), userID, userRole)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to count announcements")))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]int{"count": count}, "")
}

// MarkSeen handles marking an announcement as seen
func (rs *AnnouncementsResource) MarkSeen(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	announcementID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	if err := rs.announcementService.MarkSeen(r.Context(), userID, announcementID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to mark announcement as seen")))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement marked as seen")
}

// MarkDismissed handles marking an announcement as dismissed
func (rs *AnnouncementsResource) MarkDismissed(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	announcementID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
	if !ok {
		return
	}

	if err := rs.announcementService.MarkDismissed(r.Context(), userID, announcementID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to mark announcement as dismissed")))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement dismissed")
}
