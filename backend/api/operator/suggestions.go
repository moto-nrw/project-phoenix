package operator

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// SuggestionsResource handles operator suggestions endpoints
type SuggestionsResource struct {
	suggestionsService platformSvc.OperatorSuggestionsService
}

// NewSuggestionsResource creates a new suggestions resource
func NewSuggestionsResource(suggestionsService platformSvc.OperatorSuggestionsService) *SuggestionsResource {
	return &SuggestionsResource{
		suggestionsService: suggestionsService,
	}
}

// SuggestionResponse represents a suggestion in the response
type SuggestionResponse struct {
	ID               int64              `json:"id"`
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	Status           string             `json:"status"`
	Score            int                `json:"score"`
	Upvotes          int                `json:"upvotes"`
	Downvotes        int                `json:"downvotes"`
	AuthorName       string             `json:"author_name"`
	CreatedAt        string             `json:"created_at"`
	UpdatedAt        string             `json:"updated_at"`
	CommentCount     int                `json:"comment_count,omitempty"`
	UnreadCount      int                `json:"unread_count,omitempty"`
	OperatorComments []*CommentResponse `json:"operator_comments,omitempty"`
}

// CommentResponse represents a comment in the response (shared between operator and user APIs)
type CommentResponse struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	AuthorName string `json:"author_name"`
	AuthorType string `json:"author_type"`
	IsInternal bool   `json:"is_internal"`
	CreatedAt  string `json:"created_at"`
}

// UpdateStatusRequest represents the status update request body
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// Bind validates the update status request
func (req *UpdateStatusRequest) Bind(r *http.Request) error {
	return nil
}

// AddCommentRequest represents the add comment request body
type AddCommentRequest struct {
	Content    string `json:"content"`
	IsInternal bool   `json:"is_internal"`
}

// Bind validates the add comment request
func (req *AddCommentRequest) Bind(r *http.Request) error {
	return nil
}

// ListSuggestions handles listing all suggestions for operators
func (rs *SuggestionsResource) ListSuggestions(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorAccountID := int64(claims.ID)

	status := r.URL.Query().Get("status")
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "created_at"
	}

	posts, err := rs.suggestionsService.ListAllPosts(r.Context(), operatorAccountID, status, sortBy)
	if err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	responses := make([]SuggestionResponse, 0, len(posts))
	for _, post := range posts {
		responses = append(responses, SuggestionResponse{
			ID:           post.ID,
			Title:        post.Title,
			Description:  post.Description,
			Status:       post.Status,
			Score:        post.Score,
			Upvotes:      post.Upvotes,
			Downvotes:    post.Downvotes,
			CommentCount: post.CommentCount,
			UnreadCount:  post.UnreadCount,
			AuthorName:   post.AuthorName,
			CreatedAt:    post.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    post.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Suggestions retrieved successfully")
}

// GetSuggestion handles getting a single suggestion with comments
func (rs *SuggestionsResource) GetSuggestion(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorAccountID := int64(claims.ID)

	id, err := parseID(r, "id")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	post, comments, err := rs.suggestionsService.GetPost(r.Context(), id, operatorAccountID)
	if err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	commentResponses := make([]*CommentResponse, 0, len(comments))
	for _, comment := range comments {
		commentResponses = append(commentResponses, &CommentResponse{
			ID:         comment.ID,
			Content:    comment.Content,
			AuthorName: comment.AuthorName,
			AuthorType: comment.AuthorType,
			IsInternal: comment.IsInternal,
			CreatedAt:  comment.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	response := SuggestionResponse{
		ID:               post.ID,
		Title:            post.Title,
		Description:      post.Description,
		Status:           post.Status,
		Score:            post.Score,
		Upvotes:          post.Upvotes,
		Downvotes:        post.Downvotes,
		AuthorName:       post.AuthorName,
		CreatedAt:        post.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:        post.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		CommentCount:     len(comments),
		UnreadCount:      post.UnreadCount,
		OperatorComments: commentResponses,
	}

	common.Respond(w, r, http.StatusOK, response, "Suggestion retrieved successfully")
}

// UpdateStatus handles updating a suggestion's status
func (rs *SuggestionsResource) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	id, err := parseID(r, "id")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	req := &UpdateStatusRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.Status == "" {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("status is required")))
		return
	}

	clientIP := getClientIP(r)

	if err := rs.suggestionsService.UpdatePostStatus(r.Context(), id, req.Status, operatorID, clientIP); err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Status updated successfully")
}

// AddComment handles adding a comment to a suggestion
func (rs *SuggestionsResource) AddComment(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	postID, err := parseID(r, "id")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	req := &AddCommentRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.Content == "" {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("content is required")))
		return
	}

	comment := &suggestions.Comment{
		PostID:     postID,
		AuthorID:   operatorID,
		Content:    req.Content,
		IsInternal: req.IsInternal,
	}

	clientIP := getClientIP(r)

	if err := rs.suggestionsService.AddComment(r.Context(), comment, clientIP); err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, nil, "Comment added successfully")
}

// DeleteComment handles deleting a comment
func (rs *SuggestionsResource) DeleteComment(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	commentID, err := parseID(r, "commentId")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	clientIP := getClientIP(r)

	if err := rs.suggestionsService.DeleteComment(r.Context(), commentID, operatorID, clientIP); err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Comment deleted successfully")
}

// MarkCommentsRead marks all comments on a suggestion as read for the operator
func (rs *SuggestionsResource) MarkCommentsRead(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorAccountID := int64(claims.ID)

	postID, err := parseID(r, "id")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if err := rs.suggestionsService.MarkCommentsRead(r.Context(), operatorAccountID, postID); err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// GetUnreadCount returns the total number of unread comments across all suggestions
func (rs *SuggestionsResource) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorAccountID := int64(claims.ID)

	count, err := rs.suggestionsService.GetTotalUnreadCount(r.Context(), operatorAccountID)
	if err != nil {
		common.RenderError(w, r, SuggestionsErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved successfully")
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
