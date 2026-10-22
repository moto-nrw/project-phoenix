package owners

import (
	"context"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

func NewParentGuardianLister(db transactionruntime.Database, memberships func(context.Context, []int64, []int64) (map[int64][]int64, error)) recordports.GuardianLister {
	source := peopleCompose.NewMessageableGuardians(db.Pool(), guardianLinkView.NewReader(), memberships)
	return func(ctx context.Context, studentID int64) ([]*communication.ParentGuardianRecipient, error) {
		rows, err := source.ListGuardiansForStudent(ctx, studentID)
		if rows == nil {
			return nil, err
		}
		out := make([]*communication.ParentGuardianRecipient, len(rows))
		for i, row := range rows {
			if row != nil {
				out[i] = &communication.ParentGuardianRecipient{AccountID: row.AccountID, Name: row.Name, RelationshipType: row.RelationshipType, IsPrimary: row.IsPrimary, PortalLocale: row.PortalLocale}
			}
		}
		return out, err
	}
}

// NewParentReachableStudents lists the school's children with a guardian
// NewParentGuardianLister would offer, over the same People Directory rule.
func NewParentReachableStudents(db transactionruntime.Database, memberships func(context.Context, []int64, []int64) (map[int64][]int64, error)) recordports.ReachableStudentLister {
	return peopleCompose.NewMessageableGuardians(db.Pool(), guardianLinkView.NewReader(), memberships).ListStudentIDsWithMessageableGuardian
}
