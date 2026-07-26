package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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
	LinkURL                 *string    `json:"link_url,omitempty"`
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
		LinkURL:                 item.LinkURL,
		RequiresAcknowledgement: item.RequiresAcknowledgement,
		SchoolName:              item.SchoolName,
		PublishedAt:             item.PublishedAt,
		ExpiresAt:               item.ExpiresAt,
		Read:                    item.ReadAt != nil,
		Acknowledged:            item.AcknowledgedAt != nil,
	}
}

func parseAnnouncementID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
}

// stampRequest is the read/acknowledge body. published_at is the timestamp the
// client loaded with the feed item; the service rejects the request if it no
// longer matches the live announcement (a since-corrected/republished one), so
// a stale tab cannot record a read/ack for wording the guardian never saw.
type stampRequest struct {
	PublishedAt *time.Time `json:"published_at"`
}

// parseStampBody decodes the required published_at version token. A missing,
// unparseable or zero timestamp is a client error (the feed always supplies it).
func parseStampBody(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	var req stampRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return time.Time{}, false
	}
	if req.PublishedAt == nil || req.PublishedAt.IsZero() {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("published_at is required")))
		return time.Time{}, false
	}
	return *req.PublishedAt, true
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
	publishedAt, ok := parseStampBody(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.MarkAnnouncementRead(r.Context(), accountID, announcementID, publishedAt); err != nil {
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
	publishedAt, ok := parseStampBody(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.AcknowledgeAnnouncement(r.Context(), accountID, announcementID, publishedAt); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"acknowledged": true}, "Announcement acknowledged")
}
