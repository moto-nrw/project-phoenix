package suggestions

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Author type constants
const (
	AuthorTypeOperator = "operator"
	AuthorTypeUser     = "user"
)

// Comment represents a comment on a suggestion post
type Comment struct {
	base.Model `bun:"schema:suggestions,table:comments"`
	base.TenantModel
	PostID     int64      `bun:"post_id,notnull" json:"post_id"`
	AuthorID   int64      `bun:"author_id,notnull" json:"author_id"`
	AuthorType string     `bun:"author_type,notnull" json:"author_type"`
	Content    string     `bun:"content,notnull" json:"content"`
	DeletedAt  *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Resolved at query time, not stored
	AuthorName string `bun:"author_name,scanonly" json:"author_name,omitempty"`
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
	if len(c.Content) > bodyMaxLength {
		return errors.New("content must not exceed 5000 characters")
	}
	return nil
}

// isValidAuthorType checks if an author type string is valid
func isValidAuthorType(authorType string) bool {
	return authorType == AuthorTypeOperator || authorType == AuthorTypeUser
}
