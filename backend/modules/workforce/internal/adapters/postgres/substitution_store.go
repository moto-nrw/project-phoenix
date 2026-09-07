package postgres

import (
	"context"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const (
	tableGroupSubstitution = "education.group_substitution"
	aliasGroupSubstitution = "group_substitution"
)

type groupSubstitutionRow struct {
	bun.BaseModel     `bun:"table:education.group_substitution,alias:group_substitution"`
	ID                int64        `bun:"id,pk,autoincrement"`
	TenantID          int64        `bun:"tenant_id,notnull"`
	TargetType        string       `bun:"target_type,notnull"`
	GroupID           int64        `bun:"group_id,notnull"`
	RegularStaffID    *int64       `bun:"regular_staff_id"`
	SubstituteStaffID int64        `bun:"substitute_staff_id,notnull"`
	StartDate         calendarDate `bun:"start_date,notnull,type:date"`
	EndDate           calendarDate `bun:"end_date,notnull,type:date"`
	Reason            string       `bun:"reason"`
	CreatedAt         time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (s *Store) FindGroupSubstitution(ctx context.Context, id int64, lock bool) (domain.GroupSubstitution, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.GroupSubstitution{}, false, domain.OperationStats{}, err
	}
	row := &groupSubstitutionRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableGroupSubstitution+` AS "group_substitution"`).
		Where(`"group_substitution".id = ?`, id), aliasGroupSubstitution, tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find group substitution")
	if err != nil || !found {
		return domain.GroupSubstitution{}, found, stats, err
	}
	return groupSubstitutionToDomain(*row), true, stats, nil
}

func (s *Store) ListGroupSubstitutions(ctx context.Context, filter domain.GroupSubstitutionFilter) ([]domain.GroupSubstitution, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []groupSubstitutionRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableGroupSubstitution+` AS "group_substitution"`), aliasGroupSubstitution, tenantID)
	if filter.TenantID > 0 {
		query = query.Where(`"group_substitution".tenant_id = ?`, filter.TenantID)
	}
	if filter.GroupID > 0 {
		query = query.Where(`"group_substitution".group_id = ?`, filter.GroupID)
	}
	if filter.GroupIDs != nil {
		if len(filter.GroupIDs) == 0 {
			return []domain.GroupSubstitution{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"group_substitution".group_id IN (?)`, bun.List(filter.GroupIDs))
	}
	if filter.SubstituteStaffID > 0 {
		query = query.Where(`"group_substitution".substitute_staff_id = ?`, filter.SubstituteStaffID)
	}
	if filter.RegularStaffID > 0 {
		query = query.Where(`"group_substitution".regular_staff_id = ?`, filter.RegularStaffID)
	}
	if filter.StaffID > 0 {
		query = query.Where(`("group_substitution".regular_staff_id = ? OR "group_substitution".substitute_staff_id = ?)`, filter.StaffID, filter.StaffID)
	}
	if filter.TargetType != "" {
		query = query.Where(`"group_substitution".target_type = ?`, filter.TargetType)
	}
	if filter.On != "" {
		query = query.Where(`"group_substitution".start_date <= ? AND "group_substitution".end_date >= ?`, filter.On, filter.On)
	}
	if filter.OverlapTo != "" {
		query = query.Where(`"group_substitution".start_date <= ?`, filter.OverlapTo)
	}
	if filter.OverlapFrom != "" {
		query = query.Where(`"group_substitution".end_date >= ?`, filter.OverlapFrom)
	}
	if filter.EndsOnOrAfter != "" {
		query = query.Where(`"group_substitution".end_date >= ?`, filter.EndsOnOrAfter)
	}
	if filter.ReasonContains != "" {
		query = query.Where(`"group_substitution".reason ILIKE ?`, "%"+filter.ReasonContains+"%")
	}
	query = query.OrderExpr(`"group_substitution".id ASC`)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	stats, err := scanAll(ctx, query, "list group substitutions")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.GroupSubstitution, 0, len(rows))
	for _, row := range rows {
		result = append(result, groupSubstitutionToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CreateGroupSubstitution(ctx context.Context, value domain.GroupSubstitution) (domain.GroupSubstitution, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.GroupSubstitution{}, domain.OperationStats{}, err
	}
	row := groupSubstitutionFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableGroupSubstitution).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolation(err) {
			return domain.GroupSubstitution{}, stats, &domain.ConflictError{Kind: domain.ErrGroupSubstitutionExists, Cause: err}
		}
		return domain.GroupSubstitution{}, stats, fmt.Errorf("workforce postgres: insert group substitution: %w", err)
	}
	stats.Rows = 1
	return groupSubstitutionToDomain(*row), stats, nil
}

func (s *Store) UpdateGroupSubstitution(ctx context.Context, value domain.GroupSubstitution) (domain.GroupSubstitution, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.GroupSubstitution{}, false, domain.OperationStats{}, err
	}
	row := groupSubstitutionFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableGroupSubstitution+` AS "group_substitution"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*")
	if tenantID > 0 {
		query = query.Where(`"group_substitution".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolation(err) {
			return domain.GroupSubstitution{}, false, stats, &domain.ConflictError{Kind: domain.ErrGroupSubstitutionExists, Cause: err}
		}
		return domain.GroupSubstitution{}, false, stats, fmt.Errorf("workforce postgres: update group substitution: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.GroupSubstitution{}, false, stats, fmt.Errorf("workforce postgres: update group substitution rows affected: %w", err)
	}
	if affected != 1 {
		return domain.GroupSubstitution{}, false, stats, nil
	}
	stats.Rows = affected
	return groupSubstitutionToDomain(*row), true, stats, nil
}

func (s *Store) DeleteGroupSubstitution(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*groupSubstitutionRow)(nil)).
		ModelTableExpr(tableGroupSubstitution+` AS "group_substitution"`).
		Where(`"group_substitution".id = ?`, id), aliasGroupSubstitution, tenantID)
	return execAffected(ctx, query, "delete group substitution")
}

func (s *Store) DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*groupSubstitutionRow)(nil)).
		ModelTableExpr(tableGroupSubstitution+` AS "group_substitution"`).
		Where(`("group_substitution".regular_staff_id = ? OR "group_substitution".substitute_staff_id = ?)`, staffID, staffID).
		Where(`"group_substitution".end_date >= ?`, from), aliasGroupSubstitution, tenantID)
	stats, err := execAffected(ctx, query, "delete group substitutions for staff")
	return stats.Rows, stats, err
}

func groupSubstitutionFromDomain(value domain.GroupSubstitution) *groupSubstitutionRow {
	return &groupSubstitutionRow{
		ID: value.ID, TenantID: value.TenantID, TargetType: value.TargetType, GroupID: value.GroupID,
		RegularStaffID: value.RegularStaffID, SubstituteStaffID: value.SubstituteStaffID,
		StartDate: calendarDate(value.StartDate), EndDate: calendarDate(value.EndDate), Reason: value.Reason,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func groupSubstitutionToDomain(row groupSubstitutionRow) domain.GroupSubstitution {
	return domain.GroupSubstitution{
		ID: row.ID, TenantID: row.TenantID, TargetType: row.TargetType, GroupID: row.GroupID,
		RegularStaffID: row.RegularStaffID, SubstituteStaffID: row.SubstituteStaffID,
		StartDate: string(row.StartDate), EndDate: string(row.EndDate), Reason: row.Reason,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
