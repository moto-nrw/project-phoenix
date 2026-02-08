package suggestions

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
)

// Resource defines the suggestions API resource
type Resource struct {
	SuggestionsService suggestionsSvc.Service
}

// NewResource creates a new suggestions resource
func NewResource(suggestionsService suggestionsSvc.Service) *Resource {
	return &Resource{
		SuggestionsService: suggestionsService,
	}
}

// Router returns a configured router for suggestion endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// List and read
		r.With(authorize.RequiresPermission(permissions.SuggestionsList)).Get("/", rs.listPosts)
		r.With(authorize.RequiresPermission(permissions.SuggestionsList)).Get("/unread-count", rs.getUnreadCount)
		r.With(authorize.RequiresPermission(permissions.SuggestionsRead)).Get("/{id}", rs.getPost)

		// Create
		r.With(authorize.RequiresPermission(permissions.SuggestionsCreate)).Post("/", rs.createPost)

		// Update and delete (ownership enforced in service layer)
		r.With(authorize.RequiresPermission(permissions.SuggestionsUpdate)).Put("/{id}", rs.updatePost)
		r.With(authorize.RequiresPermission(permissions.SuggestionsDelete)).Delete("/{id}", rs.deletePost)

		// Voting
		r.With(authorize.RequiresPermission(permissions.SuggestionsCreate)).Post("/{id}/vote", rs.vote)
		r.With(authorize.RequiresPermission(permissions.SuggestionsCreate)).Delete("/{id}/vote", rs.removeVote)

		// Comments
		r.Route("/{id}/comments", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SuggestionsRead)).Get("/", rs.listComments)
			r.With(authorize.RequiresPermission(permissions.SuggestionsCreate)).Post("/", rs.createComment)
			r.With(authorize.RequiresPermission(permissions.SuggestionsCreate)).Post("/read", rs.markCommentsRead)
			r.With(authorize.RequiresPermission(permissions.SuggestionsDelete)).Delete("/{commentId}", rs.deleteComment)
		})
	})

	return r
}

// listPosts handles listing all suggestion posts
func (rs *Resource) listPosts(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "score"
	}

	posts, err := rs.SuggestionsService.ListPosts(r.Context(), accountID, sortBy)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	responses := make([]PostResponse, 0, len(posts))
	for _, post := range posts {
		responses = append(responses, newPostResponse(post))
	}

	common.Respond(w, r, http.StatusOK, responses, "Suggestions retrieved successfully")
}

// getPost handles getting a single suggestion post
func (rs *Resource) getPost(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	id, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	post, err := rs.SuggestionsService.GetPost(r.Context(), id, accountID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPostResponse(post), "Suggestion retrieved successfully")
}

// createPost handles creating a new suggestion post
func (rs *Resource) createPost(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	req := &CreatePostRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	post := &suggestions.Post{
		Title:       req.Title,
		Description: req.Description,
		AuthorID:    accountID,
	}

	if err := rs.SuggestionsService.CreatePost(r.Context(), post); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Fetch the created post with author name for response
	created, err := rs.SuggestionsService.GetPost(r.Context(), post.ID, accountID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newPostResponse(created), "Suggestion created successfully")
}

// updatePost handles updating a suggestion post
func (rs *Resource) updatePost(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	id, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	req := &UpdatePostRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	post := &suggestions.Post{
		Title:       req.Title,
		Description: req.Description,
	}
	post.ID = id

	if err := rs.SuggestionsService.UpdatePost(r.Context(), post, accountID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Fetch updated post with author name for response
	updated, err := rs.SuggestionsService.GetPost(r.Context(), id, accountID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPostResponse(updated), "Suggestion updated successfully")
}

// deletePost handles deleting a suggestion post
func (rs *Resource) deletePost(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	id, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if err := rs.SuggestionsService.DeletePost(r.Context(), id, accountID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Suggestion deleted successfully")
}

// vote handles casting or changing a vote on a post
func (rs *Resource) vote(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	id, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	req := &VoteRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	post, err := rs.SuggestionsService.Vote(r.Context(), id, accountID, req.Direction)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPostResponse(post), "Vote recorded successfully")
}

// removeVote handles removing a user's vote from a post
func (rs *Resource) removeVote(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	id, err := parsePostID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	post, err := rs.SuggestionsService.RemoveVote(r.Context(), id, accountID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPostResponse(post), "Vote removed successfully")
}

// getUnreadCount returns the total number of unread comments across all suggestions
func (rs *Resource) getUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	accountID := int64(claims.ID)

	count, err := rs.SuggestionsService.GetTotalUnreadCount(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]int{"unread_count": count}, "Unread count retrieved successfully")
}

// parsePostID extracts and validates the post ID from the URL
func parsePostID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		return 0, errors.New("suggestion ID is required")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid suggestion ID")
	}
	return id, nil
}
