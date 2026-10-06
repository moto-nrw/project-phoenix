package compose_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/fileformat"
	importAPI "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound"
	importCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestUploadRefusalsAnswerWithRegisteredCodes pins the codes the import
// pages word their message from (#2517): what is wrong with the file, with
// the missing columns or the bad row as values.
func TestUploadRefusalsAnswerWithRegisteredCodes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, module := testutil.SetupImportModule(t, db)
	resource := importAPI.NewResource(importAPI.Dependencies{Students: module.Import, Staff: module.StaffImport, ClassList: module.ClassListImport, Files: fileformat.Decoder{}, Runtime: importCompose.HTTPRuntime(db, module.PeopleDirectory, module.Membership, nil)})
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Upload", "Admin")

	cases := []struct {
		name     string
		filename string
		content  string
		code     string
		details  map[string]any
	}{
		{"missing columns", "list.csv", "Vorname\nAnna\n", "import.file_columns_missing", map[string]any{"columns": "nachname, klasse"}},
		{"no data rows", "list.csv", "Vorname,Nachname,Klasse\n", "import.file_no_rows", nil},
		{"wrong type", "list.txt", "Vorname,Nachname,Klasse\nAnna,B,1a\n", "import.file_type_invalid", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewMultipartRequest(t, http.MethodPost, "/class-list-entries/preview", "file", tc.filename, tc.content, adminBearer(t, account.ID))
			rr := testutil.ExecuteRequestForTest(t, resource.Router(), req)
			require.Equal(t, http.StatusBadRequest, rr.Code, "%s", rr.Body.String())
			var body struct {
				Code    string         `json:"code"`
				Details map[string]any `json:"details"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body.Code)
			if tc.details != nil {
				assert.Equal(t, tc.details, body.Details)
			}
		})
	}
}
