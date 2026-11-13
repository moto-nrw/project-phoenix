package enrollmenthttp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The export lists children in German dictionary order: an accent, an umlaut
// or a lowercase particle no longer pushes a child behind Z (#4949).
func TestOrderedExportEntries_UsesGermanDictionaryOrder(t *testing.T) {
	t.Parallel()
	child := func(last string) ExportChildRow {
		return ExportChildRow{Child: &capability.RequestChild{FirstName: "Kim", LastName: last, Status: capability.ChildStatusApproved}}
	}
	data := &PhaseExport{
		Phase: &capability.Phase{Name: "P"},
		Rows: []ExportRequestRow{
			{Request: &capability.Request{SubmittedAt: time.Now()}, Children: []ExportChildRow{child("Zimmermann"), child("Özdemir")}},
			{Request: &capability.Request{SubmittedAt: time.Now()}, Children: []ExportChildRow{child("adler"), child("von Bergen")}},
			{Request: &capability.Request{SubmittedAt: time.Now()}, Children: []ExportChildRow{child("Becker"), child("Ábel")}},
		},
	}
	entries := orderedExportEntries(data)
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.child.Child.LastName
	}
	assert.Equal(t, []string{"Ábel", "adler", "Becker", "Özdemir", "von Bergen", "Zimmermann"}, got)
}
