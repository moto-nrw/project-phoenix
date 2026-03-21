package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrganization_Validate_Valid(t *testing.T) {
	o := &Organization{Name: "Stadt Altenberge", Slug: "stadt-altenberge"}
	assert.NoError(t, o.Validate())
}

func TestOrganization_Validate_EmptyName(t *testing.T) {
	o := &Organization{Name: "", Slug: "test"}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestOrganization_Validate_EmptySlug(t *testing.T) {
	o := &Organization{Name: "Test", Slug: ""}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestOrganization_Validate_InvalidSlugFormat(t *testing.T) {
	o := &Organization{Name: "Test", Slug: "UPPER_CASE"}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slug must contain only lowercase")
}

func TestOrganization_Validate_ReservedSlug(t *testing.T) {
	for _, slug := range []string{"www", "api", "operator"} {
		t.Run(slug, func(t *testing.T) {
			o := &Organization{Name: "Test Org", Slug: slug}
			err := o.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "reserved for infrastructure use")
		})
	}
}

func TestOrganization_Validate_TrimsAndLowercases(t *testing.T) {
	o := &Organization{Name: "  Test Org  ", Slug: "  My-Slug  "}
	err := o.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Test Org", o.Name)
	assert.Equal(t, "my-slug", o.Slug)
}
