package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func (s *Store) StaffMergeMemberships(ctx context.Context, schoolID int64, lock bool) ([]schoolmembership.Staff, error) {
	if schoolID <= 0 {
		return nil, errors.New("staff person merge requires administrative school scope")
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	if !scope.AcrossSchools {
		return nil, errors.New("staff person merge requires an administrative transaction")
	}
	rows := []staffRow{}
	query := db.NewSelect().TableExpr(`users.staff_school_memberships AS "staff"`).ColumnExpr(`staff.*`).Where(`staff.tenant_id = ?`, schoolID).OrderExpr(`staff.id ASC`)
	if lock {
		query = query.For("UPDATE OF staff")
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	result := make([]schoolmembership.Staff, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffToPublic(row))
	}
	return result, nil
}

func (s *Store) ReassignStaffPerson(ctx context.Context, input schoolmembership.StaffPersonReassignment) error {
	schoolID, staffID, donorID, keeperID := input.SchoolID, input.StaffID, input.DonorPersonID, input.KeeperPersonID
	rows, err := s.StaffMergeMemberships(ctx, schoolID, true)
	if err != nil {
		return err
	}
	var donor, keeper *schoolmembership.Staff
	for i := range rows {
		if rows[i].PersonID == donorID {
			if donor != nil {
				return errors.New("ambiguous donor employment")
			}
			donor = &rows[i]
		}
		if rows[i].PersonID == keeperID {
			if keeper != nil {
				return errors.New("ambiguous keeper employment")
			}
			keeper = &rows[i]
		}
	}
	if donor == nil || keeper == nil || donor.ID != staffID || (donor.DeletedAt == nil) == (keeper.DeletedAt == nil) || donorID == keeperID {
		return errors.New("staff person merge no longer eligible")
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`UPDATE users.staff_school_memberships SET person_id = ? WHERE tenant_id = ? AND id = ? AND person_id = ?`, keeperID, schoolID, staffID, donorID).Exec(ctx)
	return err
}
