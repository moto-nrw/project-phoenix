package messaging

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// mountGroupThreadRoutes adds the group threads (#4784): the OGS writes to a
// group, class or AG; parents answer in their child's conversation.
func (rs *Resource) mountGroupThreadRoutes(r chi.Router, read, withTx func(http.Handler) http.Handler) {
	if rs.Groups == nil {
		return
	}
	r.With(read, withTx).Get("/group-threads", rs.listGroupThreads)
	r.With(read, withTx).Get("/group-threads/audience", rs.previewGroupAudience)
	r.With(read, withTx).Post("/group-threads", rs.createGroupThread)
	r.With(read, withTx).Get("/group-threads/{groupThreadId}", rs.getGroupThread)
	r.With(read, withTx).Post("/group-threads/{groupThreadId}/messages", rs.postGroupMessage)
	r.With(read, withTx).Post("/group-threads/{groupThreadId}/close", rs.closeGroupThread)
}

// GroupTargetBody names a group thread target. target_id is the group or AG
// id, or the class name for a class.
type GroupTargetBody struct {
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	TargetLabel string `json:"target_label"`
}

type CreateGroupThreadRequest struct {
	GroupTargetBody
	Title string `json:"title"`
	Body  string `json:"body"`
}

type GroupMessageRequest struct {
	Body string `json:"body"`
}

type GroupThreadResponse struct {
	ID            string     `json:"id"`
	TargetType    string     `json:"target_type"`
	TargetID      string     `json:"target_id"`
	TargetLabel   string     `json:"target_label"`
	Title         string     `json:"title"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type GroupThreadSummaryResponse struct {
	GroupThreadResponse
	LastMessageBody string               `json:"last_message_body"`
	MessageCount    int                  `json:"message_count"`
	FamilyCount     int                  `json:"family_count"`
	ReadCount       int                  `json:"read_count"`
	Readers         GroupReadersResponse `json:"readers"`
}

// GroupReadersResponse says who a group thread reached (#5200): the group,
// class or AG it went to and how many families received it. Answers of the
// parents go to each child's team conversation.
type GroupReadersResponse struct {
	TargetType     string `json:"target_type"`
	TargetLabel    string `json:"target_label"`
	RecipientCount int    `json:"recipient_count"`
}

func toGroupReaders(thread communication.ParentGroupThread, familyCount int) GroupReadersResponse {
	return GroupReadersResponse{TargetType: thread.Target.Type, TargetLabel: thread.Target.Label, RecipientCount: familyCount}
}

type GroupMessageResponse struct {
	ID         string    `json:"id"`
	SenderName string    `json:"sender_name"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type GroupChildResponse struct {
	StudentID   string `json:"student_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class,omitempty"`
	Reachable   bool   `json:"reachable"`
	Read        bool   `json:"read"`
}

type GroupThreadDetailResponse struct {
	Thread      GroupThreadResponse    `json:"thread"`
	Messages    []GroupMessageResponse `json:"messages"`
	Children    []GroupChildResponse   `json:"children"`
	FamilyCount int                    `json:"family_count"`
	ReadCount   int                    `json:"read_count"`
	Readers     GroupReadersResponse   `json:"readers"`
}

func (rs *Resource) listGroupThreads(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.Groups.ListParentGroupThreads(r.Context())
	if err != nil {
		renderGroupThreadError(w, r, err)
		return
	}
	out := make([]GroupThreadSummaryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, GroupThreadSummaryResponse{
			GroupThreadResponse: toGroupThreadResponse(row.Thread), LastMessageBody: row.LastMessageBody,
			MessageCount: row.MessageCount, FamilyCount: row.FamilyCount, ReadCount: row.ReadCount,
			Readers: toGroupReaders(row.Thread, row.FamilyCount),
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Group threads retrieved")
}

func (rs *Resource) previewGroupAudience(w http.ResponseWriter, r *http.Request) {
	target, err := parseGroupTarget(GroupTargetBody{
		TargetType: r.URL.Query().Get("target_type"), TargetID: r.URL.Query().Get("target_id"),
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	children, err := rs.Groups.PreviewParentGroupAudience(r.Context(), target)
	if err != nil {
		renderGroupThreadError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toGroupChildResponses(children), "Group audience retrieved")
}

func (rs *Resource) createGroupThread(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	target, err := parseGroupTarget(req.GroupTargetBody)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.Groups.CreateParentGroupThread(r.Context(), communication.ParentGroupThreadInput{
		Target: target, Title: req.Title, Body: req.Body,
	})
	if err != nil {
		renderGroupThreadError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toGroupThreadDetail(detail), "Group thread created")
}

func (rs *Resource) getGroupThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := parseInt64Param(w, r, "groupThreadId", "group thread")
	if !ok {
		return
	}
	detail, err := rs.Groups.GetParentGroupThread(r.Context(), threadID)
	if err != nil {
		renderGroupThreadError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toGroupThreadDetail(detail), "Group thread retrieved")
}

func (rs *Resource) postGroupMessage(w http.ResponseWriter, r *http.Request) {
	threadID, ok := parseInt64Param(w, r, "groupThreadId", "group thread")
	if !ok {
		return
	}
	var req GroupMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	detail, err := rs.Groups.PostParentGroupMessage(r.Context(), threadID, req.Body)
	if err != nil {
		renderGroupThreadError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toGroupThreadDetail(detail), "Group message sent")
}

func (rs *Resource) closeGroupThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := parseInt64Param(w, r, "groupThreadId", "group thread")
	if !ok {
		return
	}
	detail, err := rs.Groups.CloseParentGroupThread(r.Context(), threadID)
	if err != nil {
		renderGroupThreadError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toGroupThreadDetail(detail), "Group thread closed")
}

func parseGroupTarget(body GroupTargetBody) (communication.ParentGroupTarget, error) {
	target := communication.ParentGroupTarget{Type: body.TargetType, Label: body.TargetLabel}
	ref := strings.TrimSpace(body.TargetID)
	switch body.TargetType {
	case communication.ParentGroupTargetClass:
		if ref == "" {
			return target, errors.New("invalid class")
		}
		target.RefText = &ref
	case communication.ParentGroupTargetGroup, communication.ParentGroupTargetActivityGroup:
		id, err := strconv.ParseInt(ref, 10, 64)
		if err != nil || id <= 0 {
			return target, errors.New("invalid target ID")
		}
		target.RefID = &id
	default:
		return target, errors.New("invalid target type")
	}
	return target, nil
}

func renderGroupThreadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, communication.ErrParentGroupThreadNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, communication.ErrParentGroupThreadClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCommunicationGroupThreadClosed))
	case errors.Is(err, communication.ErrParentGroupThreadNoRecipients):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCommunicationGroupThreadNoRecipients))
	case errors.Is(err, communication.ErrParentGroupThreadInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderMessagingError(w, r, err)
	}
}

func toGroupThreadResponse(thread communication.ParentGroupThread) GroupThreadResponse {
	targetID := ""
	switch {
	case thread.Target.RefID != nil:
		targetID = strconv.FormatInt(*thread.Target.RefID, 10)
	case thread.Target.RefText != nil:
		targetID = *thread.Target.RefText
	}
	return GroupThreadResponse{
		ID: strconv.FormatInt(thread.ID, 10), TargetType: thread.Target.Type, TargetID: targetID,
		TargetLabel: thread.Target.Label, Title: thread.Title, ClosedAt: thread.ClosedAt,
		LastMessageAt: thread.LastMessageAt, CreatedAt: thread.CreatedAt,
	}
}

func toGroupChildResponses(children []communication.ParentGroupChild) []GroupChildResponse {
	out := make([]GroupChildResponse, 0, len(children))
	for _, child := range children {
		out = append(out, GroupChildResponse{
			StudentID: strconv.FormatInt(child.StudentID, 10), FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, Reachable: child.Reachable, Read: child.Read,
		})
	}
	return out
}

func toGroupThreadDetail(detail *communication.ParentGroupThreadDetail) GroupThreadDetailResponse {
	messages := make([]GroupMessageResponse, 0, len(detail.Messages))
	for _, message := range detail.Messages {
		messages = append(messages, GroupMessageResponse{
			ID: strconv.FormatInt(message.ID, 10), SenderName: message.SenderName,
			Body: message.Body, CreatedAt: message.CreatedAt,
		})
	}
	return GroupThreadDetailResponse{
		Thread: toGroupThreadResponse(detail.Thread), Messages: messages, Children: toGroupChildResponses(detail.Children),
		FamilyCount: detail.FamilyCount, ReadCount: detail.ReadCount,
		Readers: toGroupReaders(detail.Thread, detail.FamilyCount),
	}
}
