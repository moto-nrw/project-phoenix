package importapi

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentImportTemplate_GehweiseHeadersRoundTripThroughParser(t *testing.T) {
	t.Parallel()

	headers := getStudentImportHeaders()
	require.Contains(t, headers, "Gehweise.Mo")
	require.NotContains(t, headers, "Gehweise.Mo (alleine/bus/abholung) (optional)")

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	require.NoError(t, writer.Write(headers))
	require.NoError(t, writer.Write(toCSVStrings(getStudentImportExamples()[0])))
	writer.Flush()
	require.NoError(t, writer.Error())

	rows, err := importService.NewCSVParser().ParseStudents(strings.NewReader(buf.String()))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, map[string]string{
		"mon": "bus",
		"tue": "pickup",
		"wed": "bus",
		"thu": "alone",
		"fri": "bus",
	}, rows[0].DepartureDays)
}

func toCSVStrings(values []any) []string {
	out := make([]string, len(values))
	for i, value := range values {
		if value == nil {
			continue
		}
		out[i] = fmt.Sprint(value)
	}
	return out
}
