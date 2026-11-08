package postgres

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

// The adapter decodes the stored answers, consent flags, source metadata and
// consent evidence exactly once (ADR 0170). A corrupt or invalid stored row
// fails the whole read instead of yielding a partial batch.

func TestRequestBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"consent", "legal", "custom", "source"} {
		t.Run(field, func(t *testing.T) {
			invalid := json.RawMessage(`{"unfinished":`)
			row := requestRow{ID: 2}
			switch field {
			case "consent":
				row.ConsentFlags = invalid
			case "legal":
				row.LegalBlocksSnapshot = invalid
			case "custom":
				row.CustomData = invalid
			case "source":
				row.SourceMetadata = invalid
			}
			rows, err := requestValues([]requestRow{{ID: 1}, row})
			var syntaxError *json.SyntaxError
			require.ErrorAs(t, err, &syntaxError)
			require.Nil(t, rows, "a corrupt row must not return a successful partial batch")
		})
	}
}

func TestRequestBatchRejectsNonObjectAnswers(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`[]`, `"text"`, `1`} {
		rows, err := requestValues([]requestRow{{ID: 1}, {ID: 2, CustomData: json.RawMessage(raw)}})
		require.Error(t, err, raw)
		require.Nil(t, rows, raw)
	}
}

func TestChildBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"unfinished":`, `[]`} {
		rows, err := childValues([]requestChildRow{
			{ID: 1, DateOfBirth: "2018-04-15"},
			{ID: 2, DateOfBirth: "2018-04-15", CustomData: json.RawMessage(raw)},
		})
		require.Error(t, err, raw)
		require.Nil(t, rows, "a corrupt row must not return a successful partial batch")
	}
}

// Empty values store {} and an empty consent evidence list [], never null;
// stored null reads as empty values.
func TestRowStorageWritesEmptyObjects(t *testing.T) {
	t.Parallel()
	row, err := requestStorage(&enrollment.Request{ID: 1})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(row.ConsentFlags))
	require.JSONEq(t, `{}`, string(row.CustomData))
	require.JSONEq(t, `{}`, string(row.SourceMetadata))
	require.JSONEq(t, `[]`, string(row.LegalBlocksSnapshot))

	child, err := childStorage(&enrollment.RequestChild{ID: 1, DateOfBirth: calendar.NewDate(2018, 4, 15)})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(child.CustomData))

	stored, err := requestRow{ID: 1, ConsentFlags: json.RawMessage(`null`), CustomData: json.RawMessage(`null`), SourceMetadata: json.RawMessage(`null`), LegalBlocksSnapshot: json.RawMessage(`null`)}.value()
	require.NoError(t, err)
	require.True(t, stored.ConsentFlags.IsZero())
	require.True(t, stored.CustomData.IsZero())
	require.True(t, stored.SourceMetadata.IsZero())
	require.Empty(t, stored.LegalBlocksSnapshot)
}

func TestRowStorageRoundTripsValues(t *testing.T) {
	t.Parallel()
	var answers enrollment.FormValues
	answers.Set("allergies", "Nüsse")
	on := calendar.NewDate(2026, 9, 1)
	child := &enrollment.RequestChild{ID: 3, DateOfBirth: calendar.NewDate(2018, 4, 15), ActivateOn: &on, CustomData: answers}
	row, err := childStorage(child)
	require.NoError(t, err)
	back, err := row.value()
	require.NoError(t, err)
	require.Equal(t, child.DateOfBirth, back.DateOfBirth)
	require.Equal(t, on, *back.ActivateOn)
	value, ok := back.CustomData.Lookup("allergies")
	require.True(t, ok)
	require.Equal(t, "Nüsse", value)

	request := &enrollment.Request{ID: 4, CustomData: answers, LegalBlocksSnapshot: []enrollment.LegalBlocksSnapshotEntry{{ConsentFlags: json.RawMessage(`{"agb":true}`), FormSchemaID: 7}}}
	stored, err := requestStorage(request)
	require.NoError(t, err)
	decoded, err := stored.value()
	require.NoError(t, err)
	require.Equal(t, request.LegalBlocksSnapshot, decoded.LegalBlocksSnapshot, "the frozen consent evidence keeps its bytes")
	require.Equal(t, request.CustomData, decoded.CustomData)
}
