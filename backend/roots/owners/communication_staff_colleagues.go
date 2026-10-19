package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewCommunicationStaffColleagues binds the same People Directory lookup once.
// Its picker and predicate still share the canonical active-school staff relation.
func NewCommunicationStaffColleagues(db transactionruntime.Database, staffAccounts func(context.Context) ([]int64, error), identity peopleCompose.StaffMessageIdentity) communication.StaffColleagueReads {
	source := peopleCompose.NewMessageableStaff(db.Pool(), staffAccounts, identity)
	return communication.NewStaffColleagueReads(func(ctx context.Context, id int64) ([]*communication.AddressableStaffRecord, error) {
		rows, err := source.ListMessageableStaff(ctx, id)
		if rows == nil {
			return nil, err
		}
		result := make([]*communication.AddressableStaffRecord, len(rows))
		for i, row := range rows {
			if row != nil {
				result[i] = &communication.AddressableStaffRecord{AccountID: row.AccountID, Name: row.Name, RoleKind: row.RoleKind}
			}
		}
		return result, err
	}, source.IsMessageableStaff, source.StaffRoleKinds)
}
