package suggestions

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Status constants for suggestion posts
const (
	StatusOpen       = "open"
	StatusPlanned    = "planned"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusRejected   = "rejected"
	StatusNeedInfo   = "need_info"
)

// Post author types. They separate the two boards: staff posts belong to the
// school's suggestion board, parent posts to the parent portal's feedback
// board. Which one a transaction can see is enforced by RLS via
// app.current_actor_type (migration 1.15.241), not by the WHERE clauses here.
const (
	PostAuthorStaff  = "staff"
	PostAuthorParent = "parent"
)

// Field-length bounds for suggestion posts and comments.
const (
	// postTitleMaxLength caps a suggestion post title.
	postTitleMaxLength = 200
	// bodyMaxLength caps free-text bodies (post descriptions, comment content).
	bodyMaxLength = 5000
)

// Post represents a suggestion post from a user
type Post struct {
	base.Model `bun:"schema:suggestions,table:posts"`
	base.TenantModel
	Title       string `bun:"title,notnull" json:"title"`
	Description string `bun:"description,notnull" json:"description"`
	AuthorID    int64  `bun:"author_id,notnull" json:"author_id"`
	AuthorType  string `bun:"author_type,notnull,default:'staff'" json:"author_type"`
	Status      string `bun:"status,notnull,default:'open'" json:"status"`
	Score       int    `bun:"score,notnull,default:0" json:"score"`
	IsHidden    bool   `bun:"is_hidden,notnull,default:false" json:"is_hidden"`

	// Resolved at query time, not stored
	AuthorName   string `bun:"author_name,scanonly" json:"author_name,omitempty"`
	Upvotes      int    `bun:"upvotes,scanonly" json:"upvotes"`
	Downvotes    int    `bun:"downvotes,scanonly" json:"downvotes"`
	CommentCount int    `bun:"comment_count,scanonly" json:"comment_count"`
	UnreadCount  int    `bun:"unread_count,scanonly" json:"unread_count"`
	// IsNew indicates the operator has never viewed this post
	IsNew bool `bun:"is_new,scanonly" json:"is_new"`
	// Per-user vote direction, resolved at query time
	UserVote *string `bun:"user_vote,scanonly" json:"user_vote"`
	// School metadata, resolved at query time via JOIN to platform.schools
	SchoolName string `bun:"school_name,scanonly" json:"school_name,omitempty"`
}

// Validate ensures post data is valid
func (p *Post) Validate() error {
	p.Title = strings.TrimSpace(p.Title)
	p.Description = strings.TrimSpace(p.Description)

	if p.Title == "" {
		return errors.New("title is required")
	}
	if len(p.Title) > postTitleMaxLength {
		return errors.New("title must not exceed 200 characters")
	}
	if p.Description == "" {
		return errors.New("description is required")
	}
	if len(p.Description) > bodyMaxLength {
		return errors.New("description must not exceed 5000 characters")
	}
	if p.AuthorID <= 0 {
		return errors.New("author ID is required")
	}
	// An unset author type is the staff board, matching the column default.
	if p.AuthorType == "" {
		p.AuthorType = PostAuthorStaff
	}
	if !IsValidPostAuthorType(p.AuthorType) {
		return errors.New("author type must be 'staff' or 'parent'")
	}
	if !IsValidStatus(p.Status) {
		return errors.New("invalid status")
	}
	return nil
}

// IsValidPostAuthorType checks if a post author type string is valid.
func IsValidPostAuthorType(authorType string) bool {
	return authorType == PostAuthorStaff || authorType == PostAuthorParent
}

// IsFromParent reports whether the post was written in the parent portal.
func (p *Post) IsFromParent() bool {
	return p.AuthorType == PostAuthorParent
}

// IsValidStatus checks if a status string is valid
func IsValidStatus(status string) bool {
	switch status {
	case StatusOpen, StatusPlanned, StatusInProgress, StatusDone, StatusRejected, StatusNeedInfo:
		return true
	default:
		return false
	}
}
