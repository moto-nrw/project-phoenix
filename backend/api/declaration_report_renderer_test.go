package api

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ports' document shape, spelled out: the consumers alias the same
// unnamed structs, and base.go binding this adapter to both ports is what
// proves it satisfies them.
type (
	testReportField = struct{ Label, Value string }
	testReportBlock = struct {
		Title  string
		Fields []testReportField
	}
	testReportCard = struct {
		Title  string
		Fields []testReportField
		Blocks []testReportBlock
	}
	testReportSection = struct {
		Title string
		Cards []testReportCard
	}
	testReportDocument = struct {
		Title       string
		Subtitle    string
		GeneratedAt time.Time
		Filters     []string
		Footer      string
		Sections    []testReportSection
	}
)

func TestDeclarationReportsRenderTheMotoPDF(t *testing.T) {
	t.Parallel()
	doc := testReportDocument{
		Title: "Nachweisbericht", Subtitle: "Einverständnis \x01Zoo", GeneratedAt: time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC),
		Filters: []string{"Eine sorgeberechtigte Person genügt"}, Footer: "Vertraulich",
		Sections: []testReportSection{{Title: "Stand je Kind", Cards: []testReportCard{{
			Title:  "Lina Richter",
			Fields: []testReportField{{Label: "Stand", Value: "Zugestimmt"}},
			Blocks: []testReportBlock{{Title: "Klaus Richter", Fields: []testReportField{{Label: "Antwort", Value: "Zugestimmt\nam 27.09.2026"}}}},
		}}}},
	}

	file, err := newDeclarationReports().RenderReport(doc, "nachweis-erklaerung")
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.Equal(t, "nachweis-erklaerung.pdf", file.Filename)
	assert.True(t, bytes.HasPrefix(file.Data, []byte("%PDF")))
}

func TestDeclarationReportsStripRendererMarkers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Einverständnis Zoo", reportText("Einverständnis \x01Zoo\x02"))
}
