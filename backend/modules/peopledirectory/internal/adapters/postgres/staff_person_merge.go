package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Store) StaffMergePeople(ctx context.Context, schoolID int64, lock bool) ([]peopledirectory.StaffMergePerson, error) {
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
	rows := []struct {
		personRow
		NameKey    string
		HasStudent bool
	}{}
	query := db.NewSelect().TableExpr(`users.persons AS "person"`).
		ColumnExpr(`person.*, json_build_array(lower(person.first_name), lower(person.last_name))::text AS name_key`).
		ColumnExpr(`EXISTS (SELECT 1 FROM users.student_profiles child WHERE child.tenant_id = person.tenant_id AND child.person_id = person.id) AS has_student`).
		Where(`person.tenant_id = ? AND person.deleted_at IS NULL`, schoolID).OrderExpr(`person.id ASC`)
	if lock {
		query = query.For("UPDATE OF person")
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	result := make([]peopledirectory.StaffMergePerson, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.StaffMergePerson{Person: toDomain(row.personRow), NameKey: row.NameKey, HasStudent: row.HasStudent})
	}
	return result, nil
}

// RetireMergedStaffPerson preserves the tombstone and its historical references.
// No account or card is unlinked; an unexpected card or child blocks the repair.
func (s *Store) RetireMergedStaffPerson(ctx context.Context, schoolID, donorID, keeperID int64) error {
	people, err := s.StaffMergePeople(ctx, schoolID, true)
	if err != nil {
		return err
	}
	var donor, keeper *peopledirectory.StaffMergePerson
	for i := range people {
		if people[i].ID == donorID {
			donor = &people[i]
		}
		if people[i].ID == keeperID {
			keeper = &people[i]
		}
	}
	if donor == nil || keeper == nil || donorID == keeperID || donor.AccountID != nil || keeper.AccountID == nil || donor.TagID != nil || donor.HasStudent || keeper.HasStudent || donor.NameKey != keeper.NameKey || (donor.Birthday != "" && keeper.Birthday != "" && donor.Birthday != keeper.Birthday) {
		return errors.New("staff person merge no longer eligible")
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err = db.NewRaw(`UPDATE users.persons SET birthday = COALESCE(birthday, (SELECT birthday FROM users.persons WHERE tenant_id = ? AND id = ?)) WHERE tenant_id = ? AND id = ?`, schoolID, donorID, schoolID, keeperID).Exec(ctx); err != nil {
		return err
	}
	_, err = db.NewRaw(`UPDATE users.persons SET deleted_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`, schoolID, donorID).Exec(ctx)
	return err
}
