package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type absenceEmailStaffRecords interface {
	GetStaffContactInfo(context.Context, int64) (*users.StaffWithRoleInfo, error)
	ListStaffWithPermission(context.Context, string) ([]*users.StaffWithRoleInfo, error)
}

type absenceEmailStaffDirectory struct{ source absenceEmailStaffRecords }

func (d absenceEmailStaffDirectory) GetStaffContactInfo(ctx context.Context, id int64) (*active.AbsenceEmailContact, error) {
	row, err := d.source.GetStaffContactInfo(ctx, id)
	return absenceEmailContact(row), err
}

func (d absenceEmailStaffDirectory) ListAbsenceApprovers(ctx context.Context) ([]*active.AbsenceEmailContact, error) {
	rows, err := d.source.ListStaffWithPermission(ctx, "vacation:approve")
	if rows == nil {
		return nil, err
	}
	contacts := make([]*active.AbsenceEmailContact, len(rows))
	for i, row := range rows {
		contacts[i] = absenceEmailContact(row)
	}
	return contacts, err
}

func absenceEmailContact(row *users.StaffWithRoleInfo) *active.AbsenceEmailContact {
	if row == nil {
		return nil
	}
	return &active.AbsenceEmailContact{StaffID: row.StaffID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email}
}
