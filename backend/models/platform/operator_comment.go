package platform

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableSuggestionsOperatorComments is the schema-qualified table name
// Note: This table lives in the suggestions schema but belongs to platform domain
const tableSuggestionsOperatorComments = "suggestions.operator_comments"

// OperatorComment represents an operator's comment on a suggestion
type OperatorComment struct {
	base.Model `bun:"schema:suggestions,table:operator_comments"`
	PostID     int64  `bun:"post_id,notnull" json:"post_id"`
	OperatorID int64  `bun:"operator_id,notnull" json:"operator_id"`
	Content    string `bun:"content,notnull" json:"content"`
	IsInternal bool   `bun:"is_internal,notnull,default:false" json:"is_internal"`

	// Relations
	Operator *Operator `bun:"rel:belongs-to,join:operator_id=id" json:"operator,omitempty"`
}

func (c *OperatorComment) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableSuggestionsOperatorComments)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableSuggestionsOperatorComments)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableSuggestionsOperatorComments)
	}
	return nil
}

// TableName returns the database table name
func (c *OperatorComment) TableName() string {
	return tableSuggestionsOperatorComments
}

// Validate ensures comment data is valid
func (c *OperatorComment) Validate() error {
	c.Content = strings.TrimSpace(c.Content)

	if c.PostID <= 0 {
		return errors.New("post ID is required")
	}
	if c.OperatorID <= 0 {
		return errors.New("operator ID is required")
	}
	if c.Content == "" {
		return errors.New("content is required")
	}
	if len(c.Content) > 5000 {
		return errors.New("content must not exceed 5000 characters")
	}
	return nil
}

// GetID returns the entity's ID
func (c *OperatorComment) GetID() any {
	return c.ID
}

// GetCreatedAt returns the creation timestamp
func (c *OperatorComment) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (c *OperatorComment) GetUpdatedAt() time.Time {
	return c.UpdatedAt
}
