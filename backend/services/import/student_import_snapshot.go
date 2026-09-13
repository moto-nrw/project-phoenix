package importpkg

import (
	"encoding/json"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// SnapshotImportRows retains the distinction between an absent permission
// cell and an explicit false value without changing the public JSON response.
func (c *StudentImportConfig) SnapshotImportRows(rows []importModels.StudentImportRow) ([]importModels.StudentImportRow, json.RawMessage, error) {
	detached, encoded, err := snapshotImportRows(rows)
	if err != nil {
		return nil, nil, err
	}
	type suppliedPermissions struct {
		Primary, EmergencyContact, Pickup bool
	}
	permissions := make([][]suppliedPermissions, len(rows))
	groups := make([]*int64, len(rows))
	for i, row := range rows {
		if row.GroupID != nil {
			groupID := *row.GroupID
			detached[i].GroupID = &groupID
			groups[i] = &groupID
		}
		permissions[i] = make([]suppliedPermissions, len(row.Guardians))
		for j, guardian := range row.Guardians {
			copy := &detached[i].Guardians[j]
			copy.IsPrimarySet = guardian.IsPrimarySet
			copy.IsEmergencyContactSet = guardian.IsEmergencyContactSet
			copy.CanPickupSet = guardian.CanPickupSet
			permissions[i][j] = suppliedPermissions{guardian.IsPrimarySet, guardian.IsEmergencyContactSet, guardian.CanPickupSet}
		}
	}
	identity, err := json.Marshal(struct {
		Rows        json.RawMessage
		Permissions [][]suppliedPermissions
		Groups      []*int64
	}{encoded, permissions, groups})
	return detached, identity, err
}
