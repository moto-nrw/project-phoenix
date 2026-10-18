package api

import (
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	statisticsAPI "github.com/moto-nrw/project-phoenix/modules/statistics/http"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
)

func statisticsHTTPRuntime(renderer *lists.RendererService) statisticsAPI.Runtime {
	runtime := statisticsAPI.Runtime{
		ReportGuard: common.RequiresAllPermissions(presenceStudentsHTTP.StatisticsReportPermissions()...),
		Actor: func(r *http.Request) studentpresence.StatisticsActor {
			claims := sessiontoken.ClaimsFromCtx(r.Context())
			return studentpresence.StatisticsActor{AccountID: int64(claims.ID), Role: strings.Join(claims.Roles, ",")}
		},
	}
	if renderer != nil {
		runtime.Export = func(doc statisticsAPI.ExportDocument, format statisticsAPI.ExportFormat, filename string) (statisticsAPI.ExportFile, error) {
			file, err := renderer.Render(statisticsHTTPDocument(doc), lists.Format(format), filename)
			return statisticsAPI.ExportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, err
		}
	}
	return runtime
}

func statisticsHTTPDocument(doc statisticsAPI.ExportDocument) lists.Document {
	result := lists.Document{
		Title: doc.Title, Subtitle: doc.Subtitle, GeneratedAt: doc.GeneratedAt,
		Filters: doc.Filters, Footer: doc.Footer,
	}
	for _, column := range doc.Columns {
		result.Columns = append(result.Columns, lists.Column{ID: lists.ColumnID(column.ID), Label: column.Label})
	}
	for _, row := range doc.Rows {
		var values map[lists.ColumnID]string
		if row.Values != nil {
			values = make(map[lists.ColumnID]string, len(row.Values))
		}
		for id, value := range row.Values {
			values[lists.ColumnID(id)] = value
		}
		result.Rows = append(result.Rows, lists.Row{Values: values, GroupTitle: row.GroupTitle})
	}
	return result
}
