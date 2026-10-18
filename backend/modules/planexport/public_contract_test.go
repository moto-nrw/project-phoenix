package planexport_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/stretchr/testify/require"
)

type emptyOverview struct{ from, to planexport.Date }

func (o *emptyOverview) StaffScheduleOverview(_ context.Context, from, to planexport.Date) (*planexport.StaffScheduleOverview, error) {
	o.from, o.to = from, to
	return &planexport.StaffScheduleOverview{}, nil
}
func TestPublicDocumentContractRendersAnActualWeeklyWorkbook(t *testing.T) {
	t.Parallel()
	overview := &emptyOverview{}
	service := planexport.NewService(planexport.Dependencies{Overview: overview, Renderer: lists.NewRenderer()}, nil)
	params := planexport.Params{From: "2026-08-26", To: "2026-08-27", Template: planexport.TemplateByPerson, Variant: planexport.VariantNotice, Format: planexport.FormatXLSX}
	file, err := service.ExportDienstplan(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, planexport.Date("2026-08-24"), overview.from)
	require.Equal(t, planexport.Date("2026-08-30"), overview.to)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", file.ContentType)
	require.True(t, strings.HasSuffix(file.Filename, ".xlsx"))
	workbook, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	require.NoError(t, err)
	var xml strings.Builder
	for _, part := range workbook.File {
		if !strings.HasSuffix(part.Name, ".xml") {
			continue
		}
		reader, err := part.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		xml.Write(content)
	}
	require.Contains(t, xml.String(), "Dienstplan")
	require.Contains(t, xml.String(), "Vertraulich")
	params.To = "2026-08-20"
	rejected, err := service.ExportDienstplan(context.Background(), params)
	require.ErrorIs(t, err, planexport.ErrRangeReversed)
	require.Empty(t, rejected.Data)
}
