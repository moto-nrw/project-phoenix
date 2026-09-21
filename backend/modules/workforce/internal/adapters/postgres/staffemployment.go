package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// staffPersonnelNumberConstraint is the name the personnel-number trigger
// reports, pinned by migration 1.15.409.
const staffPersonnelNumberConstraint = "uq_staff_tenant_personnel_number"

// staffEmploymentRow is the Workforce half of a staff member. This adapter is
// the only application writer of users.staff_employment_profiles (#2753).
type staffEmploymentRow struct {
	bun.BaseModel         `bun:"table:users.staff_employment_profiles,alias:profile"`
	MembershipID          int64         `bun:"membership_id,pk"`
	TenantID              int64         `bun:"tenant_id,notnull"`
	StaffNotes            string        `bun:"staff_notes"`
	EmploymentType        *string       `bun:"employment_type"`
	WorkTimeModelID       *int64        `bun:"work_time_model_id"`
	PersonnelNumber       *string       `bun:"personnel_number"`
	RotationAnchorDate    *calendarDate `bun:"rotation_anchor_date,type:date"`
	BirthdayDisplayOptOut bool          `bun:"birthday_display_opt_out,notnull"`
}

func (row staffEmploymentRow) toDomain() domain.StaffEmployment {
	return domain.StaffEmployment{
		MembershipID: row.MembershipID, StaffNotes: row.StaffNotes, EmploymentType: row.EmploymentType,
		WorkTimeModelID: row.WorkTimeModelID, PersonnelNumber: row.PersonnelNumber,
		RotationAnchorDate: calendarDateString(row.RotationAnchorDate), BirthdayDisplayOptOut: row.BirthdayDisplayOptOut,
	}
}

func staffEmploymentTenant[Q interface{ Where(string, ...any) Q }](query Q, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"profile".tenant_id = ?`, tenantID)
	}
	return query
}

func (s *Store) StaffEmployments(ctx context.Context, membershipIDs []int64) (map[int64]domain.StaffEmployment, error) {
	result := make(map[int64]domain.StaffEmployment, len(membershipIDs))
	if len(membershipIDs) == 0 {
		return result, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []staffEmploymentRow
	if err := staffEmploymentTenant(db.NewSelect().Model(&rows).
		Where(`"profile".membership_id IN (?)`, bun.List(membershipIDs)), tenantID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("workforce postgres: list staff employment: %w", err)
	}
	for _, row := range rows {
		result[row.MembershipID] = row.toDomain()
	}
	return result, nil
}

func (s *Store) StaffOnWorkTimeModel(ctx context.Context, workTimeModelID int64) ([]int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	if err := staffEmploymentTenant(db.NewSelect().Model((*staffEmploymentRow)(nil)).
		Column("membership_id").
		Where(`"profile".work_time_model_id = ?`, workTimeModelID).
		OrderExpr(`"profile".membership_id ASC`), tenantID).
		Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("workforce postgres: list staff on work time model: %w", err)
	}
	return ids, nil
}

// LockStaffEmployment reads one profile FOR UPDATE.
func (s *Store) LockStaffEmployment(ctx context.Context, membershipID int64) (domain.StaffEmployment, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffEmployment{}, err
	}
	row := staffEmploymentRow{}
	err = staffEmploymentTenant(db.NewSelect().Model(&row).
		Where(`"profile".membership_id = ?`, membershipID).For("UPDATE"), tenantID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StaffEmployment{}, domain.ErrStaffEmploymentNotFound
	}
	if err != nil {
		return domain.StaffEmployment{}, fmt.Errorf("workforce postgres: lock staff employment: %w", err)
	}
	return row.toDomain(), nil
}

// SaveStaffEmployment creates the profile or replaces every field of it.
func (s *Store) SaveStaffEmployment(ctx context.Context, value domain.StaffEmployment) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if tenantID <= 0 {
		return errors.New("workforce postgres: tenant is required to save a staff employment profile")
	}
	row := staffEmploymentRow{
		MembershipID: value.MembershipID, TenantID: tenantID, StaffNotes: value.StaffNotes,
		EmploymentType: value.EmploymentType, WorkTimeModelID: value.WorkTimeModelID,
		PersonnelNumber: value.PersonnelNumber, RotationAnchorDate: optionalCalendarDate(value.RotationAnchorDate),
		BirthdayDisplayOptOut: value.BirthdayDisplayOptOut,
	}
	result, err := db.NewInsert().Model(&row).
		On(`CONFLICT (membership_id) DO UPDATE`).
		Set(`staff_notes = EXCLUDED.staff_notes`).
		Set(`employment_type = EXCLUDED.employment_type`).
		Set(`work_time_model_id = EXCLUDED.work_time_model_id`).
		Set(`personnel_number = EXCLUDED.personnel_number`).
		Set(`rotation_anchor_date = EXCLUDED.rotation_anchor_date`).
		Set(`birthday_display_opt_out = EXCLUDED.birthday_display_opt_out`).
		Where(`"profile".tenant_id = EXCLUDED.tenant_id`).
		Exec(ctx)
	if err != nil {
		return staffEmploymentWriteError("save", err)
	}
	// The conflict guard skips a profile another school holds under this ID
	// without an error; that is a membership the caller's school does not have.
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("workforce postgres: save staff employment: count rows: %w", err)
	}
	if rows != 1 {
		return domain.ErrStaffEmploymentNotFound
	}
	return nil
}

func (s *Store) ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error {
	return s.updateStaffEmployment(ctx, "clear staff work time model", membershipID, func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set(`work_time_model_id = NULL`)
	})
}

func (s *Store) SetStaffNotes(ctx context.Context, membershipID int64, notes string) error {
	return s.updateStaffEmployment(ctx, "set staff notes", membershipID, func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set(`staff_notes = ?`, notes)
	})
}

func (s *Store) SetStaffBirthdayDisplayOptOut(ctx context.Context, membershipID int64, optOut bool) error {
	return s.updateStaffEmployment(ctx, "set staff birthday display opt-out", membershipID, func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set(`birthday_display_opt_out = ?`, optOut)
	})
}

func (s *Store) RebaseStaffRotationAnchor(ctx context.Context, membershipIDs []int64, anchorDate string) error {
	if len(membershipIDs) == 0 {
		return nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := staffEmploymentTenant(db.NewUpdate().Model((*staffEmploymentRow)(nil)).
		Set(`rotation_anchor_date = ?`, optionalCalendarDate(anchorDate)).
		Where(`"profile".membership_id IN (?)`, bun.List(membershipIDs)), tenantID).
		Exec(ctx); err != nil {
		return fmt.Errorf("workforce postgres: rebase staff rotation anchor: %w", err)
	}
	return nil
}

func (s *Store) updateStaffEmployment(ctx context.Context, operation string, membershipID int64, set func(*bun.UpdateQuery) *bun.UpdateQuery) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	result, err := staffEmploymentTenant(set(db.NewUpdate().Model((*staffEmploymentRow)(nil))).
		Where(`"profile".membership_id = ?`, membershipID), tenantID).Exec(ctx)
	if err != nil {
		return staffEmploymentWriteError(operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("workforce postgres: %s: count rows: %w", operation, err)
	}
	if rows != 1 {
		return domain.ErrStaffEmploymentNotFound
	}
	return nil
}

func staffEmploymentWriteError(operation string, err error) error {
	if pgErr, ok := errors.AsType[pgdriver.Error](err); ok && pgErr.IntegrityViolation() && pgErr.Field('n') == staffPersonnelNumberConstraint {
		return domain.ErrPersonnelNumberTaken
	}
	return fmt.Errorf("workforce postgres: %s staff employment: %w", operation, err)
}
