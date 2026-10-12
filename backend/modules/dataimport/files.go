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
	Students(io.Reader, FileFormat, StudentFileOptions) (StudentFile, error)
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

// StudentFileOptions carries the choices the person made for an upload that
// needs them. Only an IServ user export asks for one: which user types are
// children (#1446).
type StudentFileOptions struct {
	// IServUserTypes lists the IServ "Benutzertyp" values to import. nil
	// means no choice was made yet, so the decoder preselects the types that
	// name pupils; an empty slice imports nobody.
	IServUserTypes []string
}

// StudentFile is a decoded student upload. IServ is set when the file is an
// IServ user export; its rows are then only the chosen user types.
type StudentFile struct {
	Rows  []StudentImportRow
	IServ *IServSummary
}

// IServSummary tells the preview what an IServ user export contains, so the
// person can choose which user types are children. IServ lets every school
// name its user types freely, so moto cannot know them in advance.
type IServSummary struct {
	UserTypes []IServUserType `json:"user_types"`
	// DeactivatedCount is the number of deactivated accounts left out.
	DeactivatedCount int `json:"deactivated_count"`
}

// IServUserType is one "Benutzertyp" value of an IServ export. An empty
// value is an account created by hand in IServ.
type IServUserType struct {
	Value    string `json:"value"`
	Count    int    `json:"count"`
	Selected bool   `json:"selected"`
}
