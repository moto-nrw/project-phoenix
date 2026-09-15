package dataimport

import "io"

// FileFormat identifies the validated upload format. File validation remains
// the inbound adapter's responsibility before a decoder sees the contents.
type FileFormat string

const (
	CSV  FileFormat = "csv"
	XLSX FileFormat = "xlsx"
)

// FileDecoder converts uploads into the workflow's public row values without
// exposing a spreadsheet library or mutable parser state to callers.
type FileDecoder interface {
	Students(io.Reader, FileFormat) ([]StudentImportRow, error)
	Staff(io.Reader, FileFormat) ([]StaffImportRow, error)
	ClassList(io.Reader, FileFormat) ([]ClassListEntryImportRow, error)
	OpeningBalances(io.Reader, FileFormat) ([]OpeningBalanceImportRow, error)
	Template(TemplateKind, FileFormat) ([]byte, error)
}

type TemplateKind string

const (
	StudentTemplate        TemplateKind = "student"
	StaffTemplate          TemplateKind = "staff"
	ClassListTemplate      TemplateKind = "class-list"
	OpeningBalanceTemplate TemplateKind = "opening-balance"
)
