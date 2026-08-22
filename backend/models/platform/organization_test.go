package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrganization_Validate_Valid(t *testing.T) {
	t.Parallel()

	o := &Organization{Name: "Stadt Altenberge", Slug: "stadt-altenberge"}
	assert.NoError(t, o.Validate())
}

func TestOrganization_Validate_EmptyName(t *testing.T) {
	t.Parallel()

	o := &Organization{Name: "", Slug: "test"}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestOrganization_Validate_EmptySlug(t *testing.T) {
	t.Parallel()

	o := &Organization{Name: "Test", Slug: ""}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestOrganization_Validate_InvalidSlugFormat(t *testing.T) {
	t.Parallel()

	o := &Organization{Name: "Test", Slug: "UPPER_CASE"}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slug must contain only lowercase")
}

func TestOrganization_Validate_ReservedSlug(t *testing.T) {
	t.Parallel()

	for _, slug := range []string{"www", "api", "operator", "parents", "eltern", "grafana", "pyreportal", "help", "admin", "app", "dashboard", "analytics", "status", "mail", "staging", "demo"} {
		t.Run(slug, func(t *testing.T) {
			o := &Organization{Name: "Test Org", Slug: slug}
			err := o.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "reserved for infrastructure use")
		})
	}
}

func TestOrganization_Validate_TrimsAndLowercases(t *testing.T) {
	t.Parallel()

	o := &Organization{Name: "  Test Org  ", Slug: "  My-Slug  "}
	err := o.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Test Org", o.Name)
	assert.Equal(t, "my-slug", o.Slug)
}

func TestOrganization_IsDeleted(t *testing.T) {
	t.Parallel()

	t.Run("returns false when deleted_at is nil", func(t *testing.T) {
		o := &Organization{Name: "Test", Slug: "test"}
		assert.False(t, o.IsDeleted())
	})

	t.Run("returns true when deleted_at is set", func(t *testing.T) {
		now := time.Now()
		o := &Organization{Name: "Test", Slug: "test", DeletedAt: &now}
		assert.True(t, o.IsDeleted())
	})
}
