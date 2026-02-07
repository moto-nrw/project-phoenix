package suggestions

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
)

// CreatePostRequest represents a request to create a suggestion
type CreatePostRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Bind validates the create request
func (req *CreatePostRequest) Bind(_ *http.Request) error {
	if req.Title == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > 200 {
		return errors.New("title must not exceed 200 characters")
	}
	if req.Description == "" {
		return errors.New("description is required")
	}
	if len(req.Description) > 5000 {
		return errors.New("description must not exceed 5000 characters")
	}
	return nil
}

// UpdatePostRequest represents a request to update a suggestion
type UpdatePostRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Bind validates the update request
func (req *UpdatePostRequest) Bind(_ *http.Request) error {
	if req.Title == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > 200 {
		return errors.New("title must not exceed 200 characters")
	}
	if req.Description == "" {
		return errors.New("description is required")
	}
	if len(req.Description) > 5000 {
		return errors.New("description must not exceed 5000 characters")
	}
	return nil
}

// VoteRequest represents a request to cast a vote
type VoteRequest struct {
	Direction string `json:"direction"`
}

// Bind validates the vote request
func (req *VoteRequest) Bind(_ *http.Request) error {
	if req.Direction != "up" && req.Direction != "down" {
		return errors.New("direction must be 'up' or 'down'")
	}
	return nil
}

// PostResponse represents a suggestion post in API responses
type PostResponse struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	AuthorID     int64   `json:"author_id"`
	AuthorName   string  `json:"author_name"`
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

// newPostResponse converts a model Post to a PostResponse
func newPostResponse(post *suggestions.Post) PostResponse {
	var userVote *string
	if post.UserVote != nil && *post.UserVote != "" {
		userVote = post.UserVote
	}

	return PostResponse{
		ID:           post.ID,
		Title:        post.Title,
		Description:  post.Description,
		AuthorID:     post.AuthorID,
		AuthorName:   post.AuthorName,
		Status:       post.Status,
		Score:        post.Score,
		Upvotes:      post.Upvotes,
		Downvotes:    post.Downvotes,
		CommentCount: post.CommentCount,
		UnreadCount:  post.UnreadCount,
		UserVote:     userVote,
		CreatedAt:    post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    post.UpdatedAt.Format(time.RFC3339),
	}
}
