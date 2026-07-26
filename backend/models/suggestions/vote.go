package suggestions

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Vote direction constants
const (
	DirectionUp   = "up"
	DirectionDown = "down"
)

// Vote represents a user's vote on a suggestion post
type Vote struct {
	base.Model `bun:"schema:suggestions,table:votes"`
	base.TenantModel
	PostID    int64  `bun:"post_id,notnull" json:"post_id"`
	VoterID   int64  `bun:"voter_id,notnull" json:"voter_id"`
	Direction string `bun:"direction,notnull" json:"direction"`
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
