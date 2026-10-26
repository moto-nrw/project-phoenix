package parentbilling

import "errors"

// The overview of a released run for the Kämmerei (spec #4611 user stories
// 79 and 80, #4672, ADR 0122): every issued invoice of the run with its
// positions per family, the sum of every invoice and the sum of the run, as
// a PDF, an Excel or a CSV file. It is the general layout until a Träger
// brings its own; the Elternabrechnung fills the table, Document Rendering
// renders it.

// Formats of the overview.
const (
	ExportPDF  = "pdf"
	ExportXLSX = "xlsx"
	ExportCSV  = "csv"
)

// ErrBillingRunNotReleased reports the overview of a run that has not
// issued its invoices yet.
var ErrBillingRunNotReleased = errors.New("the billing run is not released")

// ExportColumn is one column of an exported table.
type ExportColumn struct {
	ID    string
	Label string
}

// ExportTable is a list the Elternabrechnung exports: a title, the lines
// under it, the columns and one map of values per row.
type ExportTable struct {
	Title    string
	Subtitle string
	Lines    []string
	Columns  []ExportColumn
	Rows     []map[string]string
	Footer   string
}

// ExportFile is a rendered table.
type ExportFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

// TableRenderer renders an exported table as a PDF, an Excel or a CSV
// file. The root binds it to Document Rendering's list renderer; it neither
// stores the file nor decides who may read it.
type TableRenderer interface {
	RenderTable(table ExportTable, format, filenameBase string) (ExportFile, error)
}
