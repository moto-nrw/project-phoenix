package owners

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type staffSenderNames func(context.Context, int64) (*communication.StaffSenderName, error)

func (read staffSenderNames) FindStaffSenderName(ctx context.Context, id int64) (*communication.StaffSenderName, error) {
	return read(ctx, id)
}

// BindStaffSenderNames retains the same person read and original name parts.
// The workflow still owns its exact whitespace/fallback behavior.
func BindStaffSenderNames(source interface {
	FindByAccountID(context.Context, int64) (*peopleValues.DirectoryPersonSnapshot, error)
}) communication.StaffSenderNames {
	if source == nil {
		return nil
	}
	return staffSenderNames(func(ctx context.Context, id int64) (*communication.StaffSenderName, error) {
		row, err := source.FindByAccountID(ctx, id)
		if row == nil {
			return nil, err
		}
		return &communication.StaffSenderName{FirstName: row.FirstName, LastName: row.LastName}, err
	})
}
