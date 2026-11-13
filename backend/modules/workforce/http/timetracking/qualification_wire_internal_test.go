// Router-level tests for the Nachweise (#4808): kinds and the list sit behind
// staff:stammdaten, a qualification row can be assigned a kind, and a kind of
// another school stays invisible.
package timetracking

import (
	"encoding/json"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualificationIDsStayDecimalStringsOnTheWire(t *testing.T) {
	t.Parallel()

	largeID := int64(9007199254740993)
	typeID := largeID + 1
	kindEncoded, err := json.Marshal(toQualificationTypeResponse(workforce.QualificationType{ID: largeID}))
	require.NoError(t, err)
	assert.Contains(t, string(kindEncoded), `"id":"9007199254740993"`)

	entry := toQualificationEntryResponse(workforce.QualificationEntry{
		StaffID:         largeID,
		QualificationID: &largeID,
		TypeID:          &typeID,
	})
	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.JSONEq(t, `{"staff_id":"9007199254740993","qualification_id":"9007199254740993","type_id":"9007199254740994","name":"","acquired_on":null,"expires_on":null,"status":"","mandatory":false}`, string(encoded))

	var request StammdatenQualifikationenRequest
	require.NoError(t, json.Unmarshal([]byte(`{"qualifikationen":[{"id":"9007199254740993","type_id":"9007199254740994","name":"Erste Hilfe"}]}`), &request))
	require.NotNil(t, request.Qualifikationen[0].ID)
	require.NotNil(t, request.Qualifikationen[0].TypeID)
	assert.Equal(t, largeID, request.Qualifikationen[0].ID.Int64())
	assert.Equal(t, typeID, request.Qualifikationen[0].TypeID.Int64())
}

// Rule 15 (#2940): the Nachweise list and the kinds read one batch each,
// however many people and qualifications exist.
