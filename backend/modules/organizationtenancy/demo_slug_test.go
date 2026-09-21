package organizationtenancy_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoSchoolSlugIsAValidSchoolSlug(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"OGS Nord":                                              "ogs-nord-k3m9xp",
		"  Grundschule Süd / Köln-Weiß  ":                       "grundschule-sued-koeln-weiss-k3m9xp",
		"OGS »Am Mühlenbach« (Träger AWO)":                      "ogs-am-muehlenbach-traeger-awo-k3m9xp",
		"Offene Ganztagsschule an der Gemeinschaftsgrundschule": "offene-ganztagsschule-an-der-g-k3m9xp",
		"学校":  "ogs-k3m9xp",
		"---": "ogs-k3m9xp",
	} {
		slug := organizationtenancy.DemoSchoolSlug(name, "k3m9xp")
		assert.Equal(t, want, slug, name)
		require.NoError(t, organizationtenancy.ValidateSlug(slug), name)
		assert.LessOrEqual(t, len(slug), 63, "a slug is a DNS label")
	}
}
