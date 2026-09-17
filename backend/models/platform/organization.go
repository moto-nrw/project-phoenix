package platform

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Organization represents a top-level tenant organization (e.g. a school district).
type Organization struct {
	base.Model `bun:"schema:platform,table:organizations"`
	Name       string     `bun:"name,notnull" json:"name"`
	Slug       string     `bun:"slug,notnull,unique" json:"slug"`
	Active     bool       `bun:"active,notnull,default:true" json:"active"`
	DeletedAt  *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Settings   string     `bun:"settings,default:'{}'" json:"settings,omitempty"`
}
