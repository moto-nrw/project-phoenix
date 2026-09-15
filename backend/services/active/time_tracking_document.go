package active

import "time"

type TimeTrackingColumn struct {
	ID, Label string
}

type TimeTrackingRow struct {
	Values map[string]string
}

// TimeTrackingDocument is the prepared single-staff export, independent of its renderer.
type TimeTrackingDocument struct {
	Title, Subtitle, Footer string
	GeneratedAt             time.Time
	Filters                 []string
	Columns                 []TimeTrackingColumn
	Rows                    []TimeTrackingRow
}

type TimeTrackingPDFRenderer func(TimeTrackingDocument) ([]byte, error)

type TimeTrackingWorkbookRenderer func(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error)
