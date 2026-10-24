package substitutions

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

// TestClassifyKeepsRefusalDetails pins that a schedule refusal's own code,
// the values it names and its field survive the classification (#2516).
func TestClassifyKeepsRefusalDetails(t *testing.T) {
	t.Parallel()

	failure := classify(&workforce.SubstitutionOperationError{
		Target: workforce.ErrSubstitutionInvalidTarget, Code: "timetable.substitute_absent_on_date",
		Message: "Die Angaben für die Vertretung sind ungültig.",
		Details: workforce.SubstitutionRefusalValues{Date: "2026-10-06"}, Field: "substitute_staff_id",
	})
	require.Equal(t, 400, failure.Status)
	require.Equal(t, "timetable.substitute_absent_on_date", failure.Code)
	require.Equal(t, map[string]any{"date": "2026-10-06"}, failure.Details)
	require.Equal(t, "substitute_staff_id", failure.Field)
}
