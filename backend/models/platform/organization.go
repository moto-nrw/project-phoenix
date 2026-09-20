package platform

import "time"

// Organization represents a top-level tenant organization (e.g. a school district).
type Organization struct {
	ID        int64      `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	Name      string     `bun:"name,notnull" json:"name"`
	Slug      string     `bun:"slug,notnull,unique" json:"slug"`
	Active    bool       `bun:"active,notnull,default:true" json:"active"`
	DeletedAt *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Settings  string     `bun:"settings,default:'{}'" json:"settings,omitempty"`
}
