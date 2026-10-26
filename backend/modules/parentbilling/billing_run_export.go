package parentbilling

// The overview of a released run for the Kämmerei (spec #4611 user stories
// 79 and 80, #4672, ADR 0122): every issued invoice of the run with its
// positions per family, the sum of every invoice and the sum of the run, as
// a PDF, an Excel or a CSV file. It is the general layout until a Träger
// brings its own; the Elternabrechnung fills the table, Document Rendering
// renders it through the owner's rendering adapter.

// Formats of the overview.
const (
	ExportPDF  = "pdf"
	ExportXLSX = "xlsx"
	ExportCSV  = "csv"
)

// ExportFile is a rendered table.
type ExportFile struct {
	Filename    string
	ContentType string
	Data        []byte
}
