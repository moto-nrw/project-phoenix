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

	// Poll fields (#1371). response_type "none" means the other three are absent:
	// options are the answer choices, children the guardian's own children this
	// poll reaches (one answer row each), deadline the answer cut-off.
	ResponseType     string                  `json:"response_type"`
	ResponseDeadline *time.Time              `json:"response_deadline,omitempty"`
	Options          []AnnouncementOption    `json:"options,omitempty"`
	Children         []AnnouncementPollChild `json:"children,omitempty"`
}

// AnnouncementOption is one answer choice of a poll.
type AnnouncementOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// AnnouncementPollChild is one of the guardian's children the poll reaches,
// with the option ids currently selected for that child (empty = unanswered).
type AnnouncementPollChild struct {
	StudentID       string   `json:"student_id"`
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	SelectedOptions []string `json:"selected_options"`
}

func toAnnouncementResponse(item *usersModels.AnnouncementFeedItem) AnnouncementResponse {
	options := make([]AnnouncementOption, 0, len(item.Options))
	for _, o := range item.Options {
		options = append(options, AnnouncementOption{
			ID:    strconv.FormatInt(o.ID, 10),
			Label: o.Label,
		})
	}
	children := make([]AnnouncementPollChild, 0, len(item.Children))
	for _, c := range item.Children {
		selected := make([]string, 0, len(c.SelectedOptions))
		for _, id := range c.SelectedOptions {
			selected = append(selected, strconv.FormatInt(id, 10))
		}
		children = append(children, AnnouncementPollChild{
			StudentID:       strconv.FormatInt(c.StudentID, 10),
			FirstName:       c.FirstName,
			LastName:        c.LastName,
			SelectedOptions: selected,
		})
	}
	return AnnouncementResponse{
		ResponseType:            item.ResponseType,
		ResponseDeadline:        item.ResponseDeadline,
		Options:                 options,
		Children:                children,
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

// respondRequest is the poll answer body: which child the answer is for, the
// selected option ids (empty withdraws the answer), and the published_at version
// token every write on the feed carries.
type respondRequest struct {
	StudentID   string     `json:"student_id"`
	OptionIDs   []string   `json:"option_ids"`
	PublishedAt *time.Time `json:"published_at"`
}

// respondToAnnouncement records the guardian's answer to a poll for one child.
func (rs *Resource) respondToAnnouncement(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	announcementID, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	var req respondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if req.PublishedAt == nil || req.PublishedAt.IsZero() {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("published_at is required")))
		return
	}
	studentID, err := strconv.ParseInt(req.StudentID, 10, 64)
	if err != nil || studentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student_id")))
		return
	}
	optionIDs := make([]int64, 0, len(req.OptionIDs))
	for _, raw := range req.OptionIDs {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid option id")))
			return
		}
		optionIDs = append(optionIDs, id)
	}
	if err := rs.ParentService.RespondToAnnouncement(r.Context(), accountID, announcementID, studentID, optionIDs, *req.PublishedAt); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"answered": len(optionIDs) > 0}, "Answer recorded")
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
