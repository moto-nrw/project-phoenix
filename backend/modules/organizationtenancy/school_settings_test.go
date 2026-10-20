package organizationtenancy_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
)

func TestSchoolLogoURLPrefersTheLogoOverTheLoginImage(t *testing.T) {
	t.Parallel()
	for settings, want := range map[string]string{
		"":                "",
		"   ":             "",
		"{not valid json": "",
		`{"other":"x"}`:   "",
		`{"logoUrl":"https://logo","loginImageUrl":"https://login"}`: "https://logo",
		`{"loginImageUrl":"https://login"}`:                          "https://login",
		`{"logoUrl":"  https://logo  "}`:                             "https://logo",
		`{"logoUrl":42,"loginImageUrl":"https://login"}`:             "https://login",
		`{"logoUrl":"","loginImageUrl":"https://login"}`:             "https://login",
	} {
		assert.Equal(t, want, organizationtenancy.SchoolLogoURL(settings), settings)
		assert.Equal(t, want, organizationtenancy.School{Settings: settings}.LogoURL(), settings)
	}
}
