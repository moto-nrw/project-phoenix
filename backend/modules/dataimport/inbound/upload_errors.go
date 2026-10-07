package importapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// Registered codes of a refused upload (error-registry.json, #2517). This
// package does not import api/common, so it names each code once here.
const (
	codeImportFileTooLarge       = "import.file_too_large"
	codeImportFileMissing        = "import.file_missing"
	codeImportFileTypeInvalid    = "import.file_type_invalid"
	codeImportFileUnreadable     = "import.file_unreadable"
	codeImportFileNoRows         = "import.file_no_rows"
	codeImportFileColumnsMissing = "import.file_columns_missing"
	codeImportFileRowInvalid     = "import.file_row_invalid"
	codeImportModeForbidden      = "import.mode_forbidden"
)

// uploadFailure is a 400 for an upload the client must fix, with its code.
func uploadFailure(code string, cause error) Failure {
	return Failure{Status: http.StatusBadRequest, Cause: cause, Code: code}
}

// decodeFailure maps a parser error to the code that tells the person what
// to fix in the file: missing columns, the first bad row, no data rows, or a
// file that could not be read at all. The cause keeps the parser's text for
// the log.
func decodeFailure(err error) Failure {
	cause := fmt.Errorf("Datei-Fehler: %s", err.Error())
	var missing *importModels.FileMissingColumnsError
	if errors.As(err, &missing) {
		failure := uploadFailure(codeImportFileColumnsMissing, cause)
		failure.Details = map[string]any{"columns": strings.Join(missing.Columns, ", ")}
		return failure
	}
	var row *importModels.FileRowError
	if errors.As(err, &row) {
		failure := uploadFailure(codeImportFileRowInvalid, cause)
		failure.Details = map[string]any{"row": row.Row}
		return failure
	}
	var noRows *importModels.FileNoDataRowsError
	if errors.As(err, &noRows) {
		return uploadFailure(codeImportFileNoRows, cause)
	}
	return uploadFailure(codeImportFileUnreadable, cause)
}
