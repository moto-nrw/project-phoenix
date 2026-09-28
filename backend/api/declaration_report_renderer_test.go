package api

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	announcementAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
)

// One binding serves the staff report and the parent proof: both ports take
// the same document shape.
var (
	_ announcementAPI.ReportRenderer = declarationReports{}
	_ parentAPI.ReportRenderer       = declarationReports{}
)

func TestDeclarationReportsRenderTheMotoPDF(t *testing.T) {
	t.Parallel()
	doc := announcementAPI.ReportDocument{
		Title: "Nachweisbericht", Subtitle: "Einverständnis \x01Zoo", GeneratedAt: time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC),
		Filters: []string{"Zustimmen oder ablehnen"}, Footer: "Vertraulich",
		Sections: []announcementAPI.ReportSection{{Title: "Stand je Kind", Cards: []announcementAPI.ReportCard{{
			Title:  "Lina Richter",
			Fields: []announcementAPI.ReportField{{Label: "Stand", Value: "Zugestimmt"}},
			Blocks: []announcementAPI.ReportBlock{{Title: "Klaus Richter", Fields: []announcementAPI.ReportField{{Label: "Antwort", Value: "Zugestimmt\nam 27.09.2026"}}}},
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
