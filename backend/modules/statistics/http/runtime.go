package statisticshttp

import (
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// Runtime binds authentication context and rendering outside the HTTP adapter.
type Runtime struct {
	ReportGuard func(http.Handler) http.Handler
	Actor       func(*http.Request) studentpresence.StatisticsActor
	Export      func(ExportDocument, ExportFormat, string) (ExportFile, error)
}

// ExportFormat selects the statistics download format.
type ExportFormat string

const (
	formatPDF  ExportFormat = "pdf"
	formatDOCX ExportFormat = "docx"
	formatXLSX ExportFormat = "xlsx"
)

// ExportDocument is the statistics adapter's table-rendering request.
type ExportDocument struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Filters     []string
	Columns     []ExportColumn
	Rows        []ExportRow
	Footer      string
}

// ExportColumn describes one statistics table column.
type ExportColumn struct {
	ID    string
	Label string
}

// ExportRow carries formatted cells or a group heading.
type ExportRow struct {
	Values     map[string]string
	GroupTitle string
}

// ExportFile is the rendered download returned by the root binding.
type ExportFile struct {
	Data        []byte
	ContentType string
	Filename    string
}
