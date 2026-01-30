package suggestions

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Vote direction constants
const (
	DirectionUp   = "up"
	DirectionDown = "down"
)

// tableSuggestionsVotes is the schema-qualified table name
const tableSuggestionsVotes = "suggestions.votes"

// Vote represents a user's vote on a suggestion post
type Vote struct {
	base.Model `bun:"schema:suggestions,table:votes"`
	PostID     int64  `bun:"post_id,notnull" json:"post_id"`
	VoterID    int64  `bun:"voter_id,notnull" json:"voter_id"`
	Direction  string `bun:"direction,notnull" json:"direction"`
}

func (v *Vote) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableSuggestionsVotes)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableSuggestionsVotes)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableSuggestionsVotes)
	}
	return nil
}

// TableName returns the database table name
func (v *Vote) TableName() string {
	return tableSuggestionsVotes
}

// Validate ensures vote data is valid
func (v *Vote) Validate() error {
	if v.PostID <= 0 {
		return errors.New("post ID is required")
	}
	if v.VoterID <= 0 {
		return errors.New("voter ID is required")
	}
	if !IsValidDirection(v.Direction) {
		return errors.New("direction must be 'up' or 'down'")
	}
	return nil
}

// IsValidDirection checks if a vote direction is valid
func IsValidDirection(direction string) bool {
	return direction == DirectionUp || direction == DirectionDown
}

// GetID returns the entity's ID
func (v *Vote) GetID() any {
	return v.ID
}

// GetCreatedAt returns the creation timestamp
func (v *Vote) GetCreatedAt() time.Time {
	return v.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (v *Vote) GetUpdatedAt() time.Time {
	return v.UpdatedAt
}
