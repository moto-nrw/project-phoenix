package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// Nachweisarten (#4808): users.staff_qualification_types and the reminder
// progress on users.staff_qualifications.

const (
	tableStaffQualificationTypes = "users.staff_qualification_types"
	aliasStaffQualificationType  = "staff_qualification_type"
	uniqueLiveQualificationName  = "uq_staff_qualification_types_live_name"
)

type staffQualificationTypeRow struct {
	bun.BaseModel     `bun:"table:users.staff_qualification_types,alias:staff_qualification_type"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	StandardKey       *string    `bun:"standard_key"`
	Name              string     `bun:"name,notnull"`
	ValidityMonths    *int       `bun:"validity_months"`
	ReminderLeadWeeks int        `bun:"reminder_lead_weeks,notnull"`
	Mandatory         bool       `bun:"mandatory,notnull"`
	SortOrder         int        `bun:"sort_order,notnull"`
	ArchivedAt        *time.Time `bun:"archived_at"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// ListQualificationTypes returns every kind of the school: live ones first in
// their order, archived ones after.
func (s *Store) ListQualificationTypes(ctx context.Context) ([]domain.QualificationType, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffQualificationTypeRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffQualificationTypes+` AS "staff_qualification_type"`), aliasStaffQualificationType, scope).
		OrderExpr(`"staff_qualification_type".archived_at IS NOT NULL, "staff_qualification_type".sort_order ASC, "staff_qualification_type".id ASC`)
	stats, err := scanAll(ctx, query, "list staff qualification types")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.QualificationType, 0, len(rows))
	for _, row := range rows {
		result = append(result, qualificationTypeToDomain(row))
	}
	return result, stats, nil
}

// EnsureStandardQualificationTypes adds the standard kinds a school does not
// have yet; existing ones, renamed or archived, stay as they are.
func (s *Store) EnsureStandardQualificationTypes(ctx context.Context, values []domain.QualificationType) (domain.OperationStats, error) {
	if len(values) == 0 {
		return domain.OperationStats{}, nil
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if err := scope.Validate(); err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]staffQualificationTypeRow, 0, len(values))
	for _, value := range values {
		row := qualificationTypeFromDomain(value)
		row.TenantID = scope.TenantID
		rows = append(rows, *row)
	}
	query := db.NewInsert().Model(&rows).ModelTableExpr(tableStaffQualificationTypes).
		On("CONFLICT (tenant_id, standard_key) WHERE standard_key IS NOT NULL DO NOTHING")
	return execAffected(ctx, query, "ensure standard staff qualification types")
}

// InsertQualificationType adds a kind of the school; a live kind of the same
// name reports domain.ErrQualificationTypeNameTaken.
func (s *Store) InsertQualificationType(ctx context.Context, value domain.QualificationType) (domain.QualificationType, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.QualificationType{}, domain.OperationStats{}, err
	}
	if err := scope.Validate(); err != nil {
		return domain.QualificationType{}, domain.OperationStats{}, err
	}
	row := qualificationTypeFromDomain(value)
	row.TenantID = scope.TenantID
	stats, err := execAffected(ctx, db.NewInsert().Model(row).ModelTableExpr(tableStaffQualificationTypes).Returning("*"), "insert staff qualification type")
	if err != nil {
		return domain.QualificationType{}, stats, classifyQualificationTypeWrite(err)
	}
	return qualificationTypeToDomain(*row), stats, nil
}

// UpdateQualificationType rewrites name, validity, lead, mandatory flag and
// archive state of one kind; found=false when the school has no such kind.
func (s *Store) UpdateQualificationType(ctx context.Context, value domain.QualificationType) (domain.QualificationType, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.QualificationType{}, false, domain.OperationStats{}, err
	}
	row := qualificationTypeFromDomain(value)
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableStaffQualificationTypes+` AS "staff_qualification_type"`).
		Column("name", "validity_months", "reminder_lead_weeks", "mandatory", "archived_at").
		WherePK().
		Returning("*"), aliasStaffQualificationType, scope)
	stats, err := execAffected(ctx, query, "update staff qualification type")
	if err != nil {
		return domain.QualificationType{}, false, stats, classifyQualificationTypeWrite(err)
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.QualificationType{}, false, stats, nil
	}
	return qualificationTypeToDomain(*row), true, stats, nil
}

// ListLiveQualifications returns every live qualification of the school,
// for the Nachweise list and the reminders.
func (s *Store) ListLiveQualifications(ctx context.Context) ([]domain.StaffQualification, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffQualificationRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffQualifications+` AS "staff_qualification"`).
		Where(`"staff_qualification".deleted_at IS NULL`), aliasStaffQualification, scope).
		OrderExpr(`"staff_qualification".staff_id ASC, "staff_qualification".sort_order ASC, "staff_qualification".id ASC`)
	stats, err := scanAll(ctx, query, "list live staff qualifications")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return staffQualificationsToDomain(rows), stats, nil
}

// ClaimQualificationReminder records that a reminder stage went out for the
// qualification's current expiry date. It claims only once: claimed=false
// when the stage was already recorded for that date, the expiry date changed
// meanwhile or the row was retired.
func (s *Store) ClaimQualificationReminder(ctx context.Context, id int64, stage, expiresOn string) (bool, domain.OperationStats, error) {
	switch stage {
	case domain.QualificationReminderLead:
		return s.claimLeadReminder(ctx, id, expiresOn)
	case domain.QualificationReminderExpiry:
		return s.claimExpiryReminder(ctx, id, expiresOn)
	default:
		return false, domain.OperationStats{}, fmt.Errorf("workforce postgres: unknown reminder stage %q", stage)
	}
}

func (s *Store) claimLeadReminder(ctx context.Context, id int64, expiresOn string) (bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffQualificationRow)(nil)).
		ModelTableExpr(tableStaffQualifications+` AS "staff_qualification"`).
		Set("lead_reminded_for = expires_on").
		Where(`"staff_qualification".id = ?`, id).
		Where(`"staff_qualification".deleted_at IS NULL`).
		Where(`"staff_qualification".expires_on = ?`, calendarDate(expiresOn)).
		Where(`("staff_qualification".lead_reminded_for IS NULL OR "staff_qualification".lead_reminded_for <> "staff_qualification".expires_on)`), aliasStaffQualification, scope)
	stats, err := execAffected(ctx, query, "claim staff qualification lead reminder")
	return err == nil && stats.Rows == 1, stats, err
}

func (s *Store) claimExpiryReminder(ctx context.Context, id int64, expiresOn string) (bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffQualificationRow)(nil)).
		ModelTableExpr(tableStaffQualifications+` AS "staff_qualification"`).
		Set("expiry_reminded_for = expires_on").
		Where(`"staff_qualification".id = ?`, id).
		Where(`"staff_qualification".deleted_at IS NULL`).
		Where(`"staff_qualification".expires_on = ?`, calendarDate(expiresOn)).
		Where(`("staff_qualification".expiry_reminded_for IS NULL OR "staff_qualification".expiry_reminded_for <> "staff_qualification".expires_on)`), aliasStaffQualification, scope)
	stats, err := execAffected(ctx, query, "claim staff qualification expiry reminder")
	return err == nil && stats.Rows == 1, stats, err
}

func classifyQualificationTypeWrite(err error) error {
	if isUniqueViolationOn(err, uniqueLiveQualificationName) {
		return &domain.ConflictError{Kind: domain.ErrQualificationTypeNameTaken, Cause: err}
	}
	return err
}

func qualificationTypeFromDomain(value domain.QualificationType) *staffQualificationTypeRow {
	row := &staffQualificationTypeRow{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, ValidityMonths: value.ValidityMonths,
		ReminderLeadWeeks: value.ReminderLeadWeeks, Mandatory: value.Mandatory, SortOrder: value.SortOrder,
		ArchivedAt: value.ArchivedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if value.StandardKey != "" {
		key := value.StandardKey
		row.StandardKey = &key
	}
	return row
}

func qualificationTypeToDomain(row staffQualificationTypeRow) domain.QualificationType {
	value := domain.QualificationType{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, ValidityMonths: row.ValidityMonths,
		ReminderLeadWeeks: row.ReminderLeadWeeks, Mandatory: row.Mandatory, SortOrder: row.SortOrder,
		ArchivedAt: row.ArchivedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.StandardKey != nil {
		value.StandardKey = *row.StandardKey
	}
	return value
}
