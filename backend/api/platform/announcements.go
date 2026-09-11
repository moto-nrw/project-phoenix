package platform

import (
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// AnnouncementsResource handles user-facing announcements endpoints
type AnnouncementsResource struct {
	announcementService communication.Capability
}

// NewAnnouncementsResource creates a new announcements resource
func NewAnnouncementsResource(announcementService communication.Capability) *AnnouncementsResource {
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

// GetUnread handles getting unread announcements for the current user, scoped to the session tenant/org
func (rs *AnnouncementsResource) GetUnread(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	userID := int64(claims.ID)

	announcements, err := rs.announcementService.GetUnreadForUser(r.Context(), userID, claims.Roles, claims.TenantID, claims.OrgID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to retrieve announcements", err))
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

	count, err := rs.announcementService.CountUnread(r.Context(), userID, claims.Roles, claims.TenantID, claims.OrgID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to count announcements", err))
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
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to mark announcement as seen", err))
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
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to mark announcement as dismissed", err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Announcement dismissed")
}
