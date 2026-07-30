package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	suggestionsModels "github.com/moto-nrw/project-phoenix/models/suggestions"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
)

// FeedbackSchoolResponse is one option of the school picker.
type FeedbackSchoolResponse struct {
	SchoolID   string `json:"school_id"`
	SchoolName string `json:"school_name"`
}

// FeedbackPostResponse is one entry of the parent feedback board.
//
// It deliberately carries no author id: guardians are pseudonymous on this
// board, so the only identity that travels is author_name ("Elternteil" /
// "Verfasser", assigned in SQL). is_own is derived server-side from the calling
// account so the app can still offer edit and delete on the guardian's own
// entries without ever learning who wrote the others.
type FeedbackPostResponse struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	AuthorName   string  `json:"author_name"`
	IsOwn        bool    `json:"is_own"`
	Status       string  `json:"status"`
	Score        int     `json:"score"`
	Upvotes      int     `json:"upvotes"`
	Downvotes    int     `json:"downvotes"`
	CommentCount int     `json:"comment_count"`
	UnreadCount  int     `json:"unread_count"`
	UserVote     *string `json:"user_vote"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func toFeedbackPostResponse(post *suggestionsModels.Post, accountID int64) FeedbackPostResponse {
	var userVote *string
	if post.UserVote != nil && *post.UserVote != "" {
		userVote = post.UserVote
	}
	return FeedbackPostResponse{
		ID:           strconv.FormatInt(post.ID, 10),
		Title:        post.Title,
		Description:  post.Description,
		AuthorName:   post.AuthorName,
		IsOwn:        post.AuthorID == accountID,
		Status:       post.Status,
		Score:        post.Score,
		Upvotes:      post.Upvotes,
		Downvotes:    post.Downvotes,
		CommentCount: post.CommentCount,
		UnreadCount:  post.UnreadCount,
		UserVote:     userVote,
		CreatedAt:    post.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    post.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// FeedbackCommentResponse is one reply in a board thread. Same rule as the
// post: no author id, only the pseudonym resolved in SQL. author_type lets the
// app style a product-team reply differently from a guardian's.
type FeedbackCommentResponse struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	AuthorName string `json:"author_name"`
	AuthorType string `json:"author_type"`
	IsOwn      bool   `json:"is_own"`
	CreatedAt  string `json:"created_at"`
}

func toFeedbackCommentResponse(comment *suggestionsModels.Comment, accountID int64) FeedbackCommentResponse {
	return FeedbackCommentResponse{
		ID:         strconv.FormatInt(comment.ID, 10),
		Content:    comment.Content,
		AuthorName: comment.AuthorName,
		AuthorType: comment.AuthorType,
		IsOwn: comment.AuthorType == suggestionsModels.AuthorTypeParent &&
			comment.AuthorID == accountID,
		CreatedAt: comment.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// feedbackPostBody is the create/update payload.
type feedbackPostBody struct {
	SchoolID    string `json:"school_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// feedbackVoteBody is the vote payload.
type feedbackVoteBody struct {
	SchoolID  string `json:"school_id"`
	Direction string `json:"direction"`
}

// feedbackSchoolBody carries the school for actions with no other payload.
type feedbackSchoolBody struct {
	SchoolID string `json:"school_id"`
}

// feedbackCommentBody is the comment payload.
type feedbackCommentBody struct {
	SchoolID string `json:"school_id"`
	Content  string `json:"content"`
}

// feedbackSchoolFromQuery reads the addressed school from ?school_id=.
func feedbackSchoolFromQuery(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return parseFeedbackSchoolID(w, r, r.URL.Query().Get("school_id"))
}

func parseFeedbackSchoolID(w http.ResponseWriter, r *http.Request, raw string) (int64, bool) {
	schoolID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || schoolID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("school_id is required")))
		return 0, false
	}
	return schoolID, true
}

func decodeFeedbackBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return false
	}
	return true
}

func parseFeedbackPostID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "postId", "invalid feedback ID")
}

// listFeedbackSchools returns the boards the guardian may use.
func (rs *Resource) listFeedbackSchools(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	schools, err := rs.ParentService.ListFeedbackSchools(r.Context(), accountID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]FeedbackSchoolResponse, 0, len(schools))
	for _, school := range schools {
		out = append(out, FeedbackSchoolResponse{
			SchoolID:   strconv.FormatInt(school.TenantID, 10),
			SchoolName: school.SchoolName,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Feedback schools retrieved")
}

// listFeedback returns one school's parent feedback board.
func (rs *Resource) listFeedback(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	schoolID, ok := feedbackSchoolFromQuery(w, r)
	if !ok {
		return
	}
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "score"
	}
	posts, err := rs.ParentService.ListFeedback(r.Context(), accountID, schoolID, sortBy)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]FeedbackPostResponse, 0, len(posts))
	for _, post := range posts {
		out = append(out, toFeedbackPostResponse(post, accountID))
	}
	common.Respond(w, r, http.StatusOK, out, "Feedback retrieved")
}

// createFeedback posts new feedback to the product team.
func (rs *Resource) createFeedback(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	var body feedbackPostBody
	if !decodeFeedbackBody(w, r, &body) {
		return
	}
	schoolID, ok := parseFeedbackSchoolID(w, r, body.SchoolID)
	if !ok {
		return
	}
	post, err := rs.ParentService.CreateFeedback(r.Context(), accountID, schoolID, body.Title, body.Description)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toFeedbackPostResponse(post, accountID), "Feedback created")
}

// getFeedback returns a single board entry.
func (rs *Resource) getFeedback(w http.ResponseWriter, r *http.Request) {
	accountID, postID, schoolID, ok := rs.feedbackTarget(w, r)
	if !ok {
		return
	}
	post, err := rs.ParentService.GetFeedback(r.Context(), accountID, schoolID, postID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toFeedbackPostResponse(post, accountID), "Feedback retrieved")
}

// updateFeedback edits the guardian's own entry.
func (rs *Resource) updateFeedback(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	postID, ok := parseFeedbackPostID(w, r)
	if !ok {
		return
	}
	var body feedbackPostBody
	if !decodeFeedbackBody(w, r, &body) {
		return
	}
	schoolID, ok := parseFeedbackSchoolID(w, r, body.SchoolID)
	if !ok {
		return
	}
	post, err := rs.ParentService.UpdateFeedback(r.Context(), accountID, schoolID, postID, body.Title, body.Description)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toFeedbackPostResponse(post, accountID), "Feedback updated")
}

// deleteFeedback removes the guardian's own entry.
func (rs *Resource) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	accountID, postID, schoolID, ok := rs.feedbackTarget(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.DeleteFeedback(r.Context(), accountID, schoolID, postID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Feedback deleted")
}

// voteFeedback casts or changes a vote.
func (rs *Resource) voteFeedback(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	postID, ok := parseFeedbackPostID(w, r)
	if !ok {
		return
	}
	var body feedbackVoteBody
	if !decodeFeedbackBody(w, r, &body) {
		return
	}
	schoolID, ok := parseFeedbackSchoolID(w, r, body.SchoolID)
	if !ok {
		return
	}
	post, err := rs.ParentService.VoteFeedback(r.Context(), accountID, schoolID, postID, body.Direction)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toFeedbackPostResponse(post, accountID), "Vote recorded")
}

// removeFeedbackVote withdraws the guardian's vote.
func (rs *Resource) removeFeedbackVote(w http.ResponseWriter, r *http.Request) {
	accountID, postID, schoolID, ok := rs.feedbackTarget(w, r)
	if !ok {
		return
	}
	post, err := rs.ParentService.RemoveFeedbackVote(r.Context(), accountID, schoolID, postID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toFeedbackPostResponse(post, accountID), "Vote removed")
}

// listFeedbackComments returns a thread.
func (rs *Resource) listFeedbackComments(w http.ResponseWriter, r *http.Request) {
	accountID, postID, schoolID, ok := rs.feedbackTarget(w, r)
	if !ok {
		return
	}
	comments, err := rs.ParentService.ListFeedbackComments(r.Context(), accountID, schoolID, postID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]FeedbackCommentResponse, 0, len(comments))
	for _, comment := range comments {
		out = append(out, toFeedbackCommentResponse(comment, accountID))
	}
	common.Respond(w, r, http.StatusOK, out, "Comments retrieved")
}

// createFeedbackComment appends a comment.
func (rs *Resource) createFeedbackComment(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	postID, ok := parseFeedbackPostID(w, r)
	if !ok {
		return
	}
	var body feedbackCommentBody
	if !decodeFeedbackBody(w, r, &body) {
		return
	}
	schoolID, ok := parseFeedbackSchoolID(w, r, body.SchoolID)
	if !ok {
		return
	}
	if err := rs.ParentService.CreateFeedbackComment(r.Context(), accountID, schoolID, postID, body.Content); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, map[string]bool{"created": true}, "Comment created")
}

// markFeedbackCommentsRead clears a thread's unread marker.
func (rs *Resource) markFeedbackCommentsRead(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	postID, ok := parseFeedbackPostID(w, r)
	if !ok {
		return
	}
	var body feedbackSchoolBody
	if !decodeFeedbackBody(w, r, &body) {
		return
	}
	schoolID, ok := parseFeedbackSchoolID(w, r, body.SchoolID)
	if !ok {
		return
	}
	if err := rs.ParentService.MarkFeedbackCommentsRead(r.Context(), accountID, schoolID, postID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"read": true}, "Comments marked read")
}

// deleteFeedbackComment removes the guardian's own comment.
func (rs *Resource) deleteFeedbackComment(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	postID, ok := parseFeedbackPostID(w, r)
	if !ok {
		return
	}
	commentID, ok := common.ParsePositiveInt64IDWithError(w, r, "commentId", "invalid comment ID")
	if !ok {
		return
	}
	schoolID, ok := feedbackSchoolFromQuery(w, r)
	if !ok {
		return
	}
	if err := rs.ParentService.DeleteFeedbackComment(r.Context(), accountID, schoolID, postID, commentID); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Comment deleted")
}

// feedbackUnreadCount powers the portal's Feedback badge.
func (rs *Resource) feedbackUnreadCount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	count, err := rs.ParentService.FeedbackUnreadCount(r.Context(), accountID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved")
}

// feedbackTarget resolves the three values every entry-scoped handler needs.
func (rs *Resource) feedbackTarget(w http.ResponseWriter, r *http.Request) (accountID, postID, schoolID int64, ok bool) {
	if accountID, ok = rs.parentAccountID(w, r); !ok {
		return 0, 0, 0, false
	}
	if postID, ok = parseFeedbackPostID(w, r); !ok {
		return 0, 0, 0, false
	}
	if schoolID, ok = feedbackSchoolFromQuery(w, r); !ok {
		return 0, 0, 0, false
	}
	return accountID, postID, schoolID, true
}

// renderFeedbackError maps the feedback-specific service errors. It is folded
// into renderParentWriteError so every handler above keeps one error path.
func renderFeedbackError(w http.ResponseWriter, r *http.Request, err error) bool {
	var (
		postNotFound    *suggestionsSvc.PostNotFoundError
		commentNotFound *suggestionsSvc.CommentNotFoundError
		forbidden       *suggestionsSvc.ForbiddenError
		invalidData     *suggestionsSvc.InvalidDataError
	)
	switch {
	case errors.Is(err, parentService.ErrFeedbackSchoolNotAllowed):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "feedback_school_not_allowed"))
	case errors.Is(err, parentService.ErrFeedbackNotWired):
		common.RenderError(w, r, common.ErrorInternalServer(err))
	case errors.As(err, &postNotFound), errors.As(err, &commentNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.As(err, &forbidden):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "feedback_not_owned"))
	case errors.As(err, &invalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		return false
	}
	return true
}
