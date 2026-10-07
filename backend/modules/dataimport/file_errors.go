package dataimport

import (
	"fmt"
	"strings"
)

// Typed decode failures of a FileDecoder, so the upload endpoint can name
// what to fix in the file (#2517). Their texts stay the ones the parsers
// wrote before.

// FileRowError names the data row (header = row 1) a decode failure came from.
type FileRowError struct {
	Row int
	Err error
}

func (e *FileRowError) Error() string { return fmt.Sprintf("row %d: %v", e.Row, e.Err) }

func (e *FileRowError) Unwrap() error { return e.Err }

// FileMissingColumnsError lists the required header columns a file lacks.
type FileMissingColumnsError struct {
	Columns []string
}

func (e *FileMissingColumnsError) Error() string {
	return "fehlende erforderliche Spalten: " + strings.Join(e.Columns, ", ")
}

// FileNoDataRowsError reports a file with a header but no data rows, usually
// the unchanged template. Kind is "CSV" or "Excel".
type FileNoDataRowsError struct {
	Kind string
}

func (e *FileNoDataRowsError) Error() string {
	return fmt.Sprintf("die %s-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen", e.Kind)
}
