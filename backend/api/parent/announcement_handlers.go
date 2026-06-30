package parent

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// AnnouncementResponse is one parent-news feed entry. IDs are stringified per
// the int64 -> string frontend convention. read/acknowledged reflect THIS
// guardian's state. requires_acknowledgement tells the app whether to show the
// "gelesen und bestätigt" action.
type AnnouncementResponse struct {
	ID                      string     `json:"id"`
	Title                   string     `json:"title"`
	Body                    string     `json:"body"`
	Priority                string     `json:"priority"`
	RequiresAcknowledgement bool       `json:"requires_acknowledgement"`
	SchoolName              string     `json:"school_name"`
	PublishedAt             *time.Time `json:"published_at,omitempty"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	Read                    bool       `json:"read"`
	Acknowledged            bool       `json:"acknowledged"`
}

func toAnnouncementResponse(item *usersModels.AnnouncementFeedItem) AnnouncementResponse {
	return AnnouncementResponse{
		ID:                      strconv.FormatInt(item.ID, 10),
		Title:                   item.Title,
		Body:                    item.Body,
		Priority:                item.Priority,
		RequiresAcknowledgement: item.RequiresAcknowledgement,
		SchoolName:              item.SchoolName,
		PublishedAt:             item.PublishedAt,
		ExpiresAt:               item.ExpiresAt,
		Read:                    item.ReadAt != nil,
		Acknowledged:            item.AcknowledgedAt != nil,
	}
}

func parseAnnouncementID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "announcementId"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid announcement ID")))
		return 0, false
	}
	return id, true
}

// listAnnouncements returns the guardian's parent-news feed across all their
// children's (news-enabled) schools.
func (rs *Resource) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	items, err := rs.ParentService.ListAnnouncements(r.Context(), accountID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]AnnouncementResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toAnnouncementResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Announcements retrieved")
}

// unreadAnnouncementCount returns the guardian's unread feed count for the
// portal Neuigkeiten badge.
func (rs *Resource) unreadAnnouncementCount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	count, err := rs.ParentService.UnreadAnnouncementCount(r.Context(), accountID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved")
}

// markAnnouncementRead records that the guardian opened an announcement.
func (rs *Resource) markAnnouncementRead(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	announcementID, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.MarkAnnouncementRead(r.Context(), accountID, announcementID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"read": true}, "Announcement marked read")
}

// acknowledgeAnnouncement records an explicit "gelesen und bestätigt".
func (rs *Resource) acknowledgeAnnouncement(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	announcementID, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.AcknowledgeAnnouncement(r.Context(), accountID, announcementID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"acknowledged": true}, "Announcement acknowledged")
}
