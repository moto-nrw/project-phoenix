package suggestions

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Author type constants
const (
	AuthorTypeOperator = "operator"
	AuthorTypeUser     = "user"
)

// tableSuggestionsComments is the schema-qualified table name
const tableSuggestionsComments = "suggestions.comments"

// Comment represents a comment on a suggestion post
type Comment struct {
	base.Model `bun:"schema:suggestions,table:comments"`
	PostID     int64      `bun:"post_id,notnull" json:"post_id"`
	AuthorID   int64      `bun:"author_id,notnull" json:"author_id"`
	AuthorType string     `bun:"author_type,notnull" json:"author_type"`
	Content    string     `bun:"content,notnull" json:"content"`
	IsInternal bool       `bun:"is_internal,notnull,default:false" json:"is_internal"`
	DeletedAt  *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Resolved at query time, not stored
	AuthorName string `bun:"author_name,scanonly" json:"author_name,omitempty"`
}

func (c *Comment) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableSuggestionsComments)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableSuggestionsComments)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableSuggestionsComments)
	}
	return nil
}

// TableName returns the database table name
func (c *Comment) TableName() string {
	return tableSuggestionsComments
}

// Validate ensures comment data is valid
func (c *Comment) Validate() error {
	c.Content = strings.TrimSpace(c.Content)

	if c.PostID <= 0 {
		return errors.New("post ID is required")
	}
	if c.AuthorID <= 0 {
		return errors.New("author ID is required")
	}
	if !isValidAuthorType(c.AuthorType) {
		return errors.New("author type must be 'operator' or 'user'")
	}
	if c.Content == "" {
		return errors.New("content is required")
	}
	if len(c.Content) > 5000 {
		return errors.New("content must not exceed 5000 characters")
	}
	// Only operators can create internal comments
	if c.IsInternal && c.AuthorType != AuthorTypeOperator {
		return errors.New("only operators can create internal comments")
	}
	return nil
}

// isValidAuthorType checks if an author type string is valid
func isValidAuthorType(authorType string) bool {
	return authorType == AuthorTypeOperator || authorType == AuthorTypeUser
}

// GetID returns the entity's ID
func (c *Comment) GetID() any {
	return c.ID
}

// GetCreatedAt returns the creation timestamp
func (c *Comment) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (c *Comment) GetUpdatedAt() time.Time {
	return c.UpdatedAt
}
