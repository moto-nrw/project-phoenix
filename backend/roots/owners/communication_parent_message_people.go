package owners

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// BindParentMessagePeople uses the same owner reads and original predicates.
func BindParentMessagePeople(source interface {
	StudentRows() peoplePorts.StudentRepository
	FindByAccountID(context.Context, int64) (*peopleValues.DirectoryPersonSnapshot, error)
}) communication.ParentMessagePeople {
	if source == nil {
		return nil
	}
	return communication.NewParentMessagePeople(func(ctx context.Context, id int64) (*communication.ParentMessageStudent, error) {
		row, err := source.StudentRows().FindByID(ctx, id)
		if row == nil {
			return nil, err
		}
		return &communication.ParentMessageStudent{Alumnus: row.IsAlumnus(), AuthorizationStudent: row.IsAuthorizationStudent()}, err
	}, BindStaffSenderNames(source))
}

// BindParentMessageGuardianProfiles preserves raw contact facts and errors from
// the original account-scoped lookup. Locale and mail policy stay in messaging.
func BindParentMessageGuardianProfiles(source interface {
	FindByAccountID(context.Context, int64) (*peopleValues.GuardianProfileSnapshot, error)
}) communication.ParentMessageGuardianProfiles {
	if source == nil {
		return nil
	}
	return communication.NewParentMessageGuardianProfiles(func(ctx context.Context, id int64) (*communication.ParentMessageGuardianContact, error) {
		row, err := source.FindByAccountID(ctx, id)
		if row == nil {
			return nil, err
		}
		return &communication.ParentMessageGuardianContact{FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, PortalLocale: row.PortalLocale}, err
	})
}
