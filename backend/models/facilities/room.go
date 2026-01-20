package facilities

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Room represents a physical room in a facility
type Room struct {
	base.TenantModel `bun:"schema:facilities,table:rooms"`
	Name             string  `bun:"name,notnull,unique" json:"name"`
	Building         string  `bun:"building" json:"building,omitempty"`
	Floor            *int    `bun:"floor" json:"floor,omitempty"`
	Capacity         *int    `bun:"capacity" json:"capacity,omitempty"`
	Category         *string `bun:"category" json:"category,omitempty"`
	Color            *string `bun:"color" json:"color,omitempty"`
}

// TableName returns the database table name
func (r *Room) TableName() string {
	return "facilities.rooms"
}

// BeforeAppendModel lets us modify query before it's executed
func (r *Room) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`facilities.rooms AS "room"`)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(`facilities.rooms`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`facilities.rooms AS "room"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`facilities.rooms AS "room"`)
	}
	return nil
}

// Validate ensures room data is valid
func (r *Room) Validate() error {
	if r.Name == "" {
		return errors.New("room name is required")
	}

	// Trim spaces from name
	r.Name = strings.TrimSpace(r.Name)

	// Validate capacity is non-negative (if provided)
	if r.Capacity != nil && *r.Capacity < 0 {
		return errors.New("capacity cannot be negative")
	}

	// Validate color is a valid hex color (if provided)
	if r.Color != nil && *r.Color != "" {
		// Add # prefix if missing
		if !strings.HasPrefix(*r.Color, "#") {
			color := "#" + *r.Color
			r.Color = &color
		}

		// Validate hex color format (#RRGGBB or #RGB)
		hexColorPattern := regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)
		if !hexColorPattern.MatchString(*r.Color) {
			return errors.New("invalid color format, must be a valid hex color")
		}
	}

	return nil
}

// IsAvailable checks if the room is available for a given capacity
func (r *Room) IsAvailable(requiredCapacity int) bool {
	// Capacity is optional (see migration 1.1.5). Treat rooms without a
	// specified capacity as available for neutral/zero-capacity requests so
	// that devices still list them instead of filtering everything out.
	if r.Capacity == nil {
		return requiredCapacity <= 0
	}
	return *r.Capacity >= requiredCapacity
}

// GetFullName returns the building and room name combined
func (r *Room) GetFullName() string {
	if r.Building != "" {
		return r.Building + " - " + r.Name
	}
	return r.Name
}

// GetID returns the entity's ID
func (r *Room) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *Room) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *Room) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}
