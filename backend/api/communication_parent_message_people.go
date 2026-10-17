package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	people "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
)

// BindParentMessagePeople uses the same owner reads and original predicates.
func BindParentMessagePeople(source interface {
	GetStudentByID(context.Context, int64) (*people.Student, error)
	FindByAccountID(context.Context, int64) (*people.Person, error)
}) communication.ParentMessagePeople {
	if source == nil {
		return nil
	}
	return communication.NewParentMessagePeople(func(ctx context.Context, id int64) (*communication.ParentMessageStudent, error) {
		row, err := source.GetStudentByID(ctx, id)
		if row == nil {
			return nil, err
		}
		return &communication.ParentMessageStudent{Alumnus: row.IsAlumnus(), AuthorizationStudent: row.IsAuthorizationStudent()}, err
	}, BindStaffSenderNames(source))
}

// BindParentMessageGuardianProfiles preserves raw contact facts and errors from
// the original account-scoped lookup. Locale and mail policy stay in messaging.
func BindParentMessageGuardianProfiles(source interface {
	FindByAccountID(context.Context, int64) (*people.GuardianProfile, error)
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
