package users

import (
	"context"

	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

var _ directory.DirectoryPersonReads = (*personRepository)(nil)

// PersonSnapshotByID preserves the existing query, context and error chain;
// this store only projects its complete result into the native owner facade.
func (r *personRepository) PersonSnapshotByID(ctx context.Context, id int64) (*directory.DirectoryPersonSnapshot, error) {
	row, err := r.FindByID(ctx, id)
	return NativeDirectoryPerson(row), err
}
func NativeDirectoryPerson(row *models.Person) *directory.DirectoryPersonSnapshot {
	if row == nil {
		return nil
	}
	result := &directory.DirectoryPersonSnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FirstName: row.FirstName, LastName: row.LastName, Birthday: row.Birthday, TagID: row.TagID, AccountID: row.AccountID, DeletedAt: row.DeletedAt}
	if row.Account != nil {
		value := directory.PersonAccountSnapshot(*row.Account)
		result.Account = &value
	}
	if row.RFIDCard != nil {
		value := directory.PersonRFIDSnapshot(*row.RFIDCard)
		result.RFIDCard = &value
	}
	return result
}
