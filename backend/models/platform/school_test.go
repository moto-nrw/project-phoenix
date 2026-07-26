package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validSchool() *School {
	return &School{
		OrganizationID: 1,
		Name:           "Grundschule Altenberge",
		Slug:           "gs-altenberge",
		Subdomain:      "gs-altenberge",
		Email:          "info@gs-altenberge.de",
	}
}

func TestSchool_Validate_Valid(t *testing.T) {
	assert.NoError(t, validSchool().Validate())
}

func TestSchool_Validate_EmptyName(t *testing.T) {
	s := validSchool()
	s.Name = ""
	err := s.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestSchool_Validate_EmptySlug(t *testing.T) {
	s := validSchool()
	s.Slug = ""
	err := s.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestSchool_Validate_InvalidSlugFormat(t *testing.T) {
	s := validSchool()
	s.Slug = "-leading-hyphen"
	err := s.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slug must contain only lowercase")
}

func TestSchool_Validate_ReservedSlug(t *testing.T) {
	for _, slug := range []string{"www", "api", "operator", "parents", "eltern", "grafana", "pyreportal", "admin", "app", "dashboard", "analytics", "status", "mail", "staging", "demo"} {
		t.Run(slug, func(t *testing.T) {
			s := validSchool()
			s.Slug = slug
			err := s.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "slug is reserved for infrastructure use")
		})
	}
}

func TestSchool_Validate_ReservedSubdomain(t *testing.T) {
	for _, sub := range []string{"www", "api", "operator", "parents", "eltern", "grafana", "pyreportal", "admin", "app", "dashboard", "analytics", "status", "mail", "staging", "demo"} {
		t.Run(sub, func(t *testing.T) {
			s := validSchool()
			s.Subdomain = sub
			err := s.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "subdomain is reserved for infrastructure use")
		})
	}
}

func TestSchool_Validate_EmptySubdomain(t *testing.T) {
	s := validSchool()
	s.Subdomain = ""
	err := s.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subdomain is required")
}

func TestSchool_Validate_MissingOrganizationID(t *testing.T) {
	s := validSchool()
	s.OrganizationID = 0
	err := s.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization_id is required")
}

func TestSchool_Validate_TrimsAndLowercases(t *testing.T) {
	s := validSchool()
	s.Name = "  Trimmed School  "
	s.Slug = "  MY-SLUG  "
	s.Subdomain = "  MY-SUB  "
	err := s.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Trimmed School", s.Name)
	assert.Equal(t, "my-slug", s.Slug)
	assert.Equal(t, "my-sub", s.Subdomain)
}
