package test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntimeCheckpointRecordsetsReadInlineSetSizes(t *testing.T) {
	t.Parallel()
	hexSet := `'\x` + hex.EncodeToString([]byte(`[{"student_id":7},{"student_id":8},{"student_id":9}]`)) + `'`
	var recordsets RuntimeCheckpointRecordsets
	recordsets.Observe([]string{
		"SELECT * FROM jsonb_to_recordset(" + hexSet + "::jsonb) AS eligibility(student_id bigint)\nWHERE true",
		`SELECT * FROM jsonb_to_recordset('[{"name":"O''Brien"}]'::jsonb) AS eligibility(student_id bigint)`,
		`WITH a AS (SELECT * FROM jsonb_to_recordset('[]'::jsonb) AS offering(id bigint)),
		 b AS (SELECT * FROM jsonb_to_recordset('[{"id":1},{"id":2}]'::jsonb) AS booking(id bigint)) SELECT 1`,
		`SELECT * FROM jsonb_to_recordset(payload.rows) AS offering(id bigint)`,
		"SELECT 1",
	})
	assert.Equal(t, []RuntimeCheckpointRecordset{
		{Site: "jsonb_to_recordset(<set>::jsonb) AS eligibility(student_id bigint)", Calls: 2, MinRows: 1, MaxRows: 3, TotalRows: 4},
		{Site: "jsonb_to_recordset(<set>::jsonb) AS offering(id bigint)),", Calls: 1, MinRows: 0, MaxRows: 0, TotalRows: 0},
		{Site: "jsonb_to_recordset(<set>::jsonb) AS booking(id bigint)) SELECT 1", Calls: 1, MinRows: 2, MaxRows: 2, TotalRows: 2},
		{Site: "jsonb_to_recordset(<set>payload.rows) AS offering(id bigint)", Calls: 1, Unparsed: 1},
	}, recordsets.Result())
	var empty RuntimeCheckpointRecordsets
	assert.Nil(t, empty.Result())
}
