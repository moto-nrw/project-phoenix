package suggestions

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Status constants for suggestion posts
const (
	StatusOpen     = "open"
	StatusPlanned  = "planned"
	StatusDone     = "done"
	StatusRejected = "rejected"
)

// tableSuggestionsPosts is the schema-qualified table name
const tableSuggestionsPosts = "suggestions.posts"

// Post represents a suggestion post from a user
type Post struct {
	base.Model  `bun:"schema:suggestions,table:posts"`
	Title       string `bun:"title,notnull" json:"title"`
	Description string `bun:"description,notnull" json:"description"`
	AuthorID    int64  `bun:"author_id,notnull" json:"author_id"`
	Status      string `bun:"status,notnull,default:'open'" json:"status"`
	Score       int    `bun:"score,notnull,default:0" json:"score"`

	// Resolved at query time, not stored
	AuthorName string `bun:"author_name,scanonly" json:"author_name,omitempty"`
	Upvotes    int    `bun:"upvotes,scanonly" json:"upvotes"`
	Downvotes  int    `bun:"downvotes,scanonly" json:"downvotes"`
	// Per-user vote direction, resolved at query time
	UserVote *string `bun:"user_vote,scanonly" json:"user_vote"`
}

func (p *Post) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableSuggestionsPosts)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableSuggestionsPosts)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableSuggestionsPosts)
	}
	return nil
}

// TableName returns the database table name
func (p *Post) TableName() string {
	return tableSuggestionsPosts
}

// Validate ensures post data is valid
func (p *Post) Validate() error {
	p.Title = strings.TrimSpace(p.Title)
	p.Description = strings.TrimSpace(p.Description)

	if p.Title == "" {
		return errors.New("title is required")
	}
	if len(p.Title) > 200 {
		return errors.New("title must not exceed 200 characters")
	}
	if p.Description == "" {
		return errors.New("description is required")
	}
	if len(p.Description) > 5000 {
		return errors.New("description must not exceed 5000 characters")
	}
	if p.AuthorID <= 0 {
		return errors.New("author ID is required")
	}
	if !IsValidStatus(p.Status) {
		return errors.New("invalid status")
	}
	return nil
}

// IsValidStatus checks if a status string is valid
func IsValidStatus(status string) bool {
	switch status {
	case StatusOpen, StatusPlanned, StatusDone, StatusRejected:
		return true
	default:
		return false
	}
}

// GetID returns the entity's ID
func (p *Post) GetID() any {
	return p.ID
}

// GetCreatedAt returns the creation timestamp
func (p *Post) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (p *Post) GetUpdatedAt() time.Time {
	return p.UpdatedAt
}
