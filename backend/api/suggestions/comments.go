package suggestions

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
)

// CommentResponse represents a comment in API responses
type CommentResponse struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	AuthorID   int64  `json:"author_id"`
	AuthorName string `json:"author_name"`
	AuthorType string `json:"author_type"`
	CreatedAt  string `json:"created_at"`
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	Content string `json:"content"`
}

// Bind validates the create comment request
func (req *CreateCommentRequest) Bind(_ *http.Request) error {
	if req.Content == "" {
		return errors.New("content is required")
	}
	if len(req.Content) > 5000 {
		return errors.New("content must not exceed 5000 characters")
	}
	return nil
}

// listComments handles listing public comments for a suggestion post
func (rs *Resource) listComments(w http.ResponseWriter, r *http.Request) {
	postID, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	comments, err := rs.SuggestionsService.GetComments(r.Context(), postID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	responses := make([]CommentResponse, 0, len(comments))
	for _, c := range comments {
		responses = append(responses, newCommentResponse(c))
	}

	common.Respond(w, r, http.StatusOK, responses, "Comments retrieved successfully")
}

// createComment handles creating a new comment on a suggestion post
func (rs *Resource) createComment(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	postID, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	req := &CreateCommentRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	comment := &suggestions.Comment{
		PostID:   postID,
		AuthorID: accountID,
		Content:  req.Content,
	}

	if err := rs.SuggestionsService.CreateComment(r.Context(), comment); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, nil, "Comment added successfully")
}

// deleteComment handles deleting a user's own comment
func (rs *Resource) deleteComment(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	commentIDStr := chi.URLParam(r, "commentId")
	if commentIDStr == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("comment ID is required")))
		return
	}
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil || commentID <= 0 {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid comment ID")))
		return
	}

	if err := rs.SuggestionsService.DeleteComment(r.Context(), commentID, accountID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Comment deleted successfully")
}

// markCommentsRead marks all comments on a post as read for the current user
func (rs *Resource) markCommentsRead(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	postID, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if err := rs.SuggestionsService.MarkCommentsRead(r.Context(), postID, accountID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// newCommentResponse converts a model Comment to a CommentResponse
func newCommentResponse(c *suggestions.Comment) CommentResponse {
	return CommentResponse{
		ID:         c.ID,
		Content:    c.Content,
		AuthorID:   c.AuthorID,
		AuthorName: c.AuthorName,
		AuthorType: c.AuthorType,
		CreatedAt:  c.CreatedAt.Format(time.RFC3339),
	}
}
