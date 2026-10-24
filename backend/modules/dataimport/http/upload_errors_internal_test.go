package importapi

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// TestDecodeFailureNamesWhatToFix pins the code and values per parser error
// (#2517); anything unrecognised is an unreadable file.
func TestDecodeFailureNamesWhatToFix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		err     error
		code    string
		details map[string]any
	}{
		{"missing columns", &importModels.FileMissingColumnsError{Columns: []string{"Vorname", "Klasse"}}, codeImportFileColumnsMissing, map[string]any{"columns": "Vorname, Klasse"}},
		{"bad row", fmt.Errorf("wrap: %w", &importModels.FileRowError{Row: 7, Err: errors.New("bad value")}), codeImportFileRowInvalid, map[string]any{"row": 7}},
		{"no rows", &importModels.FileNoDataRowsError{Kind: "CSV"}, codeImportFileNoRows, nil},
		{"broken file", errors.New("open excel file: zip: not a valid zip file"), codeImportFileUnreadable, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failure := decodeFailure(tc.err)
			assert.Equal(t, http.StatusBadRequest, failure.Status)
			assert.Equal(t, tc.code, failure.Code)
			assert.Equal(t, tc.details, failure.Details)
			assert.Nil(t, failure.Result)
			assert.Contains(t, failure.Cause.Error(), "Datei-Fehler: ")
		})
	}
}

func TestRowErrorKeepsItsText(t *testing.T) {
	t.Parallel()
	err := &importModels.FileRowError{Row: 3, Err: errors.New("x")}
	assert.Equal(t, "row 3: x", err.Error())
}
