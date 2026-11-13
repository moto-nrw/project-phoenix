package studentdirectoryview_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/studentdirectoryview/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The return export opens the health field of a care row only with the
// school and care row of its own exported row (#3838, #6328).
func TestStudentHealthExportFieldOpensOnlyItsOwnRow(t *testing.T) {
	t.Parallel()
	reader := compose.NewReader(testpkg.TestHealthDataCipher)
	value := "Erdnussallergie"
	sealed, err := reader.SealStudentHealth(7, 11, &value)
	require.NoError(t, err)
	row := map[string]json.RawMessage{"tenant_id": json.RawMessage("7"), "membership_id": json.RawMessage("11")}

	opened, err := reader.OpenExportField("users.student_care_profiles", "health_data", row, sealed)
	require.NoError(t, err)
	require.Equal(t, value, opened)

	for name, call := range map[string]func() (string, error){
		"other school": func() (string, error) {
			return reader.OpenExportField("users.student_care_profiles", "health_data", map[string]json.RawMessage{"tenant_id": json.RawMessage("8"), "membership_id": json.RawMessage("11")}, sealed)
		},
		"other care row": func() (string, error) {
			return reader.OpenExportField("users.student_care_profiles", "health_data", map[string]json.RawMessage{"tenant_id": json.RawMessage("7"), "membership_id": json.RawMessage("12")}, sealed)
		},
		"row without care row": func() (string, error) {
			return reader.OpenExportField("users.student_care_profiles", "health_data", map[string]json.RawMessage{"tenant_id": json.RawMessage("7")}, sealed)
		},
		"foreign table": func() (string, error) { return reader.OpenExportField("users.persons", "health_data", row, sealed) },
		"unknown column": func() (string, error) {
			return reader.OpenExportField("users.student_care_profiles", "extra_info", row, sealed)
		},
		"without key ring": func() (string, error) {
			return compose.NewReader(nil).OpenExportField("users.student_care_profiles", "health_data", row, sealed)
		},
	} {
		opened, err := call()
		require.Error(t, err, name)
		require.Empty(t, opened, name)
		require.NotContains(t, err.Error(), value, name)
	}
}
