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

// Field-length bounds shared by platform tenant entities (organizations + schools).
const (
	// maxNameLen caps the display name of an organization or school.
	maxNameLen = 200
	// maxSlugLen caps the URL slug.
	maxSlugLen = 100
	// maxDNSLabelLen caps a subdomain to the DNS label limit (RFC 1035).
	maxDNSLabelLen = 63
	// maxEmailLen caps a contact email address.
	maxEmailLen = 255
)

// reservedSlugs are infrastructure subdomains that must never be used as tenant slugs or subdomains.
// Frontend equivalent: frontend/src/lib/reserved-slugs.ts — both lists MUST stay in sync.
var reservedSlugs = map[string]bool{
	// Active infrastructure (Caddy blocks / DNS records)
	"www":        true, // www.moto-app.de redirect
	"api":        true, // api.moto-app.de, api-staging, api-demo
	"operator":   true, // operator dashboard
	"parents":    true, // parents.moto-app.de — guardian portal (cross-tenant)
	"grafana":    true, // grafana.moto-app.de monitoring
	"pyreportal": true, // pyreportal.moto-app.de kiosk SPA
	"help":       true, // public /help docs — top-level app route shadows [tenant]
	// Defensive reservations (common infrastructure subdomains)
	"admin":     true,
	"app":       true,
	"dashboard": true,
	"analytics": true,
	"status":    true,
	"mail":      true,
	"staging":   true,
	"demo":      true,
}

// tablePlatformOrganizations is the schema-qualified table name
const tablePlatformOrganizations = "platform.organizations"

// Organization represents a top-level tenant organization (e.g. a school district).
type Organization struct {
	base.Model `bun:"schema:platform,table:organizations"`
	Name       string     `bun:"name,notnull" json:"name"`
	Slug       string     `bun:"slug,notnull,unique" json:"slug"`
	Active     bool       `bun:"active,notnull,default:true" json:"active"`
	DeletedAt  *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Settings   string     `bun:"settings,default:'{}'" json:"settings,omitempty"`
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
	if len(o.Name) > maxNameLen {
		return errors.New("name must not exceed 200 characters")
	}
	if o.Slug == "" {
		return errors.New("slug is required")
	}
	if len(o.Slug) > maxSlugLen {
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

// IsDeleted returns true if the organization has been soft-deleted.
func (o *Organization) IsDeleted() bool {
	return o.DeletedAt != nil
}
