package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentPersonReads binds the workflow to the owner's complete native
// person read. All workflow callers supply an int64 person ID.
func NewParentPersonReads(source peopledirectory.DirectoryPersonReads) parentportal.PersonReads {
	return parentportal.NewPersonReads(func(ctx context.Context, id int64) (*parentportal.PersonSnapshot, error) {
		row, err := source.PersonSnapshotByID(ctx, id)
		return parentPersonSnapshot(row), err
	})
}

func parentPersonSnapshot(row *peopledirectory.DirectoryPersonSnapshot) *parentportal.PersonSnapshot {
	if row == nil {
		return nil
	}
	value := &parentportal.PersonSnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FirstName: row.FirstName, LastName: row.LastName, Birthday: row.Birthday, TagID: row.TagID, AccountID: row.AccountID, DeletedAt: row.DeletedAt}
	var account *parentportal.PersonAccountSnapshot
	var card *parentportal.PersonRFIDSnapshot
	if row.Account != nil {
		mapped := parentportal.PersonAccountSnapshot(*row.Account)
		account = &mapped
	}
	if row.RFIDCard != nil {
		mapped := parentportal.PersonRFIDSnapshot(*row.RFIDCard)
		card = &mapped
	}
	return parentportal.WithPersonSnapshotRelations(value, account, card)
}
