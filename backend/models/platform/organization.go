package platform

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// slugRegex validates URL-safe slugs: lowercase alphanumeric with hyphens, no leading/trailing hyphens.
var slugRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// reservedSlugs are infrastructure subdomains that must never be used as tenant slugs or subdomains.
// Must match RESERVED_SUBDOMAINS in frontend/src/middleware.ts and RESERVED_SLUGS in [tenant]/layout.tsx.
var reservedSlugs = map[string]bool{
	"www":      true,
	"api":      true,
	"operator": true,
}

// tablePlatformOrganizations is the schema-qualified table name
const tablePlatformOrganizations = "platform.organizations"

// Organization represents a top-level tenant organization (e.g. a school district).
type Organization struct {
	base.Model `bun:"schema:platform,table:organizations"`
	Name       string `bun:"name,notnull" json:"name"`
	Slug       string `bun:"slug,notnull,unique" json:"slug"`
	Active     bool   `bun:"active,notnull,default:true" json:"active"`
	Settings   string `bun:"settings,default:'{}'" json:"settings,omitempty"`
}

func (o *Organization) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tablePlatformOrganizations)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.organizations AS "organization"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.organizations AS "organization"`)
	}
	return nil
}

// TableName returns the database table name
func (o *Organization) TableName() string {
	return tablePlatformOrganizations
}

// Validate ensures organization data is valid
func (o *Organization) Validate() error {
	o.Name = strings.TrimSpace(o.Name)
	o.Slug = strings.TrimSpace(strings.ToLower(o.Slug))

	if o.Name == "" {
		return errors.New("name is required")
	}
	if len(o.Name) > 200 {
		return errors.New("name must not exceed 200 characters")
	}
	if o.Slug == "" {
		return errors.New("slug is required")
	}
	if len(o.Slug) > 100 {
		return errors.New("slug must not exceed 100 characters")
	}
	if !slugRegex.MatchString(o.Slug) {
		return errors.New("slug must contain only lowercase letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	if reservedSlugs[o.Slug] {
		return errors.New("slug is reserved for infrastructure use")
	}
	return nil
}

// GetID returns the entity's ID
func (o *Organization) GetID() any {
	return o.ID
}

// GetCreatedAt returns the creation timestamp
func (o *Organization) GetCreatedAt() time.Time {
	return o.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (o *Organization) GetUpdatedAt() time.Time {
	return o.UpdatedAt
}
