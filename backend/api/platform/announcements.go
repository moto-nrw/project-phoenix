package platform

import (
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// AnnouncementsResource handles user-facing announcements endpoints
type AnnouncementsResource struct {
	announcementService communication.Capability
	runtime             Runtime
}

// NewAnnouncementsResource creates a new announcements resource
func NewAnnouncementsResource(announcementService communication.Capability, runtime Runtime) *AnnouncementsResource {
	return &AnnouncementsResource{
		announcementService: announcementService,
		runtime:             runtime,
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

func (rs *AnnouncementsResource) internalError(w http.ResponseWriter, r *http.Request, message string, err error) {
	rs.runtime.Failure(w, r, Failure{Message: message, Err: err})
}

func (rs *AnnouncementsResource) announcementID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return rs.runtime.IDParam(w, r, "id", "invalid announcement ID")
}

// GetUnread handles getting unread announcements for the current user, scoped to the session tenant/org
func (rs *AnnouncementsResource) GetUnread(w http.ResponseWriter, r *http.Request) {
	viewer := rs.runtime.Viewer(r)

	announcements, err := rs.announcementService.GetUnreadForUser(r.Context(), viewer.AccountID, viewer.Roles, viewer.TenantID, viewer.OrgID)
	if err != nil {
		rs.internalError(w, r, "failed to retrieve announcements", err)
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

	rs.runtime.Success(w, r, http.StatusOK, responses, "Unread announcements retrieved successfully")
}

// GetUnreadCount handles getting the count of unread announcements
func (rs *AnnouncementsResource) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	viewer := rs.runtime.Viewer(r)

	count, err := rs.announcementService.CountUnread(r.Context(), viewer.AccountID, viewer.Roles, viewer.TenantID, viewer.OrgID)
	if err != nil {
		rs.internalError(w, r, "failed to count announcements", err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, map[string]int{"count": count}, "")
}

// MarkSeen handles marking an announcement as seen
func (rs *AnnouncementsResource) MarkSeen(w http.ResponseWriter, r *http.Request) {
	viewer := rs.runtime.Viewer(r)

	announcementID, ok := rs.announcementID(w, r)
	if !ok {
		return
	}

	if err := rs.announcementService.MarkSeen(r.Context(), viewer.AccountID, announcementID); err != nil {
		rs.internalError(w, r, "failed to mark announcement as seen", err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, nil, "Announcement marked as seen")
}

// MarkDismissed handles marking an announcement as dismissed
func (rs *AnnouncementsResource) MarkDismissed(w http.ResponseWriter, r *http.Request) {
	viewer := rs.runtime.Viewer(r)

	announcementID, ok := rs.announcementID(w, r)
	if !ok {
		return
	}

	if err := rs.announcementService.MarkDismissed(r.Context(), viewer.AccountID, announcementID); err != nil {
		rs.internalError(w, r, "failed to mark announcement as dismissed", err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, nil, "Announcement dismissed")
}
