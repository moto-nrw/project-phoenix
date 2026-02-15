package platform

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tablePlatformSchools is the schema-qualified table name
const tablePlatformSchools = "platform.schools"

// School represents a single school (tenant) within an organization.
// The school ID is used as the tenant_id throughout the system.
type School struct {
	base.Model     `bun:"schema:platform,table:schools"`
	OrganizationID int64  `bun:"organization_id,notnull" json:"organization_id"`
	Name           string `bun:"name,notnull" json:"name"`
	Slug           string `bun:"slug,notnull" json:"slug"`
	Subdomain      string `bun:"subdomain,notnull,unique" json:"subdomain"`
	Active         bool   `bun:"active,notnull,default:true" json:"active"`
	Settings       string `bun:"settings,default:'{}'" json:"settings,omitempty"`
	Address        string `bun:"address" json:"address,omitempty"`
	City           string `bun:"city" json:"city,omitempty"`
	Zip            string `bun:"zip" json:"zip,omitempty"`
	Phone          string `bun:"phone" json:"phone,omitempty"`
	Email          string `bun:"email" json:"email,omitempty"`
	DevicePinHash  string `bun:"device_pin_hash" json:"-"`

	// Relations
	Organization *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization,omitempty"`
}

func (s *School) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tablePlatformSchools)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.schools AS "school"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.schools AS "school"`)
	}
	return nil
}

// TableName returns the database table name
func (s *School) TableName() string {
	return tablePlatformSchools
}

// Validate ensures school data is valid
func (s *School) Validate() error {
	s.Name = strings.TrimSpace(s.Name)
	s.Slug = strings.TrimSpace(strings.ToLower(s.Slug))
	s.Subdomain = strings.TrimSpace(strings.ToLower(s.Subdomain))

	if s.Name == "" {
		return errors.New("name is required")
	}
	if len(s.Name) > 200 {
		return errors.New("name must not exceed 200 characters")
	}
	if s.Slug == "" {
		return errors.New("slug is required")
	}
	if len(s.Slug) > 100 {
		return errors.New("slug must not exceed 100 characters")
	}
	if !slugRegex.MatchString(s.Slug) {
		return errors.New("slug must contain only lowercase letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	if s.Subdomain == "" {
		return errors.New("subdomain is required")
	}
	if len(s.Subdomain) > 63 {
		return errors.New("subdomain must not exceed 63 characters (DNS label limit)")
	}
	if !slugRegex.MatchString(s.Subdomain) {
		return errors.New("subdomain must be a valid DNS label: lowercase letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	if s.OrganizationID == 0 {
		return errors.New("organization_id is required")
	}
	if s.Email != "" && len(s.Email) > 255 {
		return errors.New("email must not exceed 255 characters")
	}
	return nil
}

// GetID returns the entity's ID
func (s *School) GetID() any {
	return s.ID
}

// GetCreatedAt returns the creation timestamp
func (s *School) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (s *School) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}
