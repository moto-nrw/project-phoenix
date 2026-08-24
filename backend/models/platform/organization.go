package platform

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
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
	"parents":    true, // parents.moto-app.de — legacy guardian portal redirect
	"eltern":     true, // eltern.moto-app.de — guardian portal (cross-tenant)
	"schule":     true, // schule.moto-app.de — school portal "moto schule" (#2207)
	"school":     true, // /school path namespace inside the App Router
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

// Organization represents a top-level tenant organization (e.g. a school district).
type Organization struct {
	base.Model `bun:"schema:platform,table:organizations"`
	Name       string     `bun:"name,notnull" json:"name"`
	Slug       string     `bun:"slug,notnull,unique" json:"slug"`
	Active     bool       `bun:"active,notnull,default:true" json:"active"`
	DeletedAt  *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Settings   string     `bun:"settings,default:'{}'" json:"settings,omitempty"`
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

// IsDeleted returns true if the organization has been soft-deleted.
func (o *Organization) IsDeleted() bool {
	return o.DeletedAt != nil
}
