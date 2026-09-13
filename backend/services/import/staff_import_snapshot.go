package importpkg

import (
	"encoding/json"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// SnapshotImportRows retains parser-only role and column-presence information
// without adding internal fields to the public JSON response.
func (c *StaffImportConfig) SnapshotImportRows(rows []importModels.StaffImportRow) ([]importModels.StaffImportRow, json.RawMessage, error) {
	detached, encoded, err := snapshotImportRows(rows)
	if err != nil {
		return nil, nil, err
	}
	type parserState struct {
		RoleID                  int64
		HasQualificationsColumn bool
	}
	states := make([]parserState, len(rows))
	for i, row := range rows {
		detached[i].RoleID = row.RoleID
		detached[i].HasQualificationsColumn = row.HasQualificationsColumn
		states[i] = parserState{row.RoleID, row.HasQualificationsColumn}
	}
	identity, err := json.Marshal(struct {
		Rows   json.RawMessage
		Parser []parserState
	}{encoded, states})
	return detached, identity, err
}
