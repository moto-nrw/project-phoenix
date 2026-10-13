package services

import (
	"context"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type absenceEmailStaffRecords interface {
	GetStaffContactInfo(context.Context, int64) (*users.StaffWithRoleInfo, error)
	ListStaffWithPermission(context.Context, string) ([]*users.StaffWithRoleInfo, error)
}

type absenceEmailStaffDirectory struct{ source absenceEmailStaffRecords }

func (d absenceEmailStaffDirectory) GetStaffContactInfo(ctx context.Context, id int64) (*workforceCompose.AbsenceEmailContact, error) {
	row, err := d.source.GetStaffContactInfo(ctx, id)
	return absenceEmailContact(row), err
}

func (d absenceEmailStaffDirectory) ListAbsenceApprovers(ctx context.Context) ([]*workforceCompose.AbsenceEmailContact, error) {
	rows, err := d.source.ListStaffWithPermission(ctx, "vacation:approve")
	if rows == nil {
		return nil, err
	}
	contacts := make([]*workforceCompose.AbsenceEmailContact, len(rows))
	for i, row := range rows {
		contacts[i] = absenceEmailContact(row)
	}
	return contacts, err
}

func absenceEmailContact(row *users.StaffWithRoleInfo) *workforceCompose.AbsenceEmailContact {
	if row == nil {
		return nil
	}
	return &workforceCompose.AbsenceEmailContact{StaffID: row.StaffID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email}
}
