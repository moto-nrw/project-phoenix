package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

// Persistence of the school's own wage types, surcharge rules and absence
// type mapping (#4807). Every statement runs in the tenant transaction of the
// context and names the tenant, on top of RLS.

const payrollWageTypeNameIndex = "uq_payroll_wage_types_active_name"

type payrollWageTypeRow struct {
	bun.BaseModel `bun:"table:active.payroll_wage_types,alias:payroll_wage_type"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	Name          string     `bun:"name,notnull"`
	Number        string     `bun:"number,notnull"`
	Unit          string     `bun:"unit,notnull"`
	ArchivedAt    *time.Time `bun:"archived_at"`
}

func (r payrollWageTypeRow) toContract() workforce.PayrollWageType {
	return workforce.PayrollWageType{ID: r.ID, Name: r.Name, Number: r.Number, Unit: r.Unit, Archived: r.ArchivedAt != nil}
}

type payrollSurchargeRuleRow struct {
	bun.BaseModel     `bun:"table:active.payroll_surcharge_rules,alias:payroll_surcharge_rule"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	Kind              string     `bun:"kind,notnull"`
	WindowStartMinute *int       `bun:"window_start_minute"`
	WindowEndMinute   *int       `bun:"window_end_minute"`
	RateType          string     `bun:"rate_type,notnull"`
	RateHundredths    int        `bun:"rate_hundredths,notnull"`
	WageTypeID        int64      `bun:"wage_type_id,notnull"`
	ArchivedAt        *time.Time `bun:"archived_at"`
}

func (r payrollSurchargeRuleRow) toContract() workforce.PayrollSurchargeRule {
	rule := workforce.PayrollSurchargeRule{
		ID: r.ID, Kind: r.Kind, RateType: r.RateType, RateHundredths: r.RateHundredths,
		WageTypeID: r.WageTypeID, Archived: r.ArchivedAt != nil,
	}
	if r.WindowStartMinute != nil && r.WindowEndMinute != nil {
		rule.WindowStartMinute, rule.WindowEndMinute = *r.WindowStartMinute, *r.WindowEndMinute
	}
	return rule
}

func surchargeRuleRow(tenantID int64, rule workforce.PayrollSurchargeRule) payrollSurchargeRuleRow {
	row := payrollSurchargeRuleRow{
		ID: rule.ID, TenantID: tenantID, Kind: rule.Kind, RateType: rule.RateType,
		RateHundredths: rule.RateHundredths, WageTypeID: rule.WageTypeID,
	}
	if rule.Kind == workforce.SurchargeKindNight {
		start, end := rule.WindowStartMinute, rule.WindowEndMinute
		row.WindowStartMinute, row.WindowEndMinute = &start, &end
	}
	return row
}

func (s *Store) payrollRulesDatabase(ctx context.Context) (bun.IDB, int64, error) {
	db, scope, err := s.database(ctx)
	if err == nil && scope.TenantID <= 0 {
		err = errors.New("payroll rules: tenant is required")
	}
	return db, scope.TenantID, err
}

// LockPayrollRules serializes the rule writes and complete snapshots of a
// school, so an archive cannot race a rule that books onto the wage type and
// a frozen rule set cannot combine separate commits.
func (s *Store) LockPayrollRules(ctx context.Context) error {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`,
		fmt.Sprintf("payroll_rules:%d", tenantID)).Exec(ctx); err != nil {
		return fmt.Errorf("lock payroll rules: %w", err)
	}
	return nil
}

// ReadPayrollRuleState loads the wage types and surcharge rules together, so
// the complete read stays within the payroll overview's fixed query budget
// when it takes the advisory lock.
func (s *Store) ReadPayrollRuleState(ctx context.Context) (workforce.PayrollRulesState, error) {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return workforce.PayrollRulesState{}, err
	}
	var configured struct {
		WageTypes json.RawMessage `bun:"wage_types"`
		Rules     json.RawMessage `bun:"surcharge_rules"`
	}
	if err := db.NewRaw(`SELECT
		COALESCE((SELECT jsonb_agg(jsonb_build_object(
			'id', id, 'name', name, 'number', number, 'unit', unit, 'archived', archived_at IS NOT NULL
		) ORDER BY id) FROM active.payroll_wage_types WHERE tenant_id = ?), '[]'::jsonb) AS wage_types,
		COALESCE((SELECT jsonb_agg(jsonb_build_object(
			'id', id, 'kind', kind, 'window_start_minute', window_start_minute, 'window_end_minute', window_end_minute,
			'rate_type', rate_type, 'rate_hundredths', rate_hundredths, 'wage_type_id', wage_type_id, 'archived', archived_at IS NOT NULL
		) ORDER BY id) FROM active.payroll_surcharge_rules WHERE tenant_id = ?), '[]'::jsonb) AS surcharge_rules`, tenantID, tenantID).
		Scan(ctx, &configured); err != nil {
		return workforce.PayrollRulesState{}, fmt.Errorf("read payroll wage types and surcharge rules: %w", err)
	}
	state := workforce.PayrollRulesState{}
	if err := json.Unmarshal(configured.WageTypes, &state.WageTypes); err != nil {
		return workforce.PayrollRulesState{}, fmt.Errorf("decode payroll wage types: %w", err)
	}
	if err := json.Unmarshal(configured.Rules, &state.Rules); err != nil {
		return workforce.PayrollRulesState{}, fmt.Errorf("decode payroll surcharge rules: %w", err)
	}
	var absenceTypes []struct {
		ID         int64  `bun:"id"`
		Name       string `bun:"name"`
		IsActive   bool   `bun:"is_active"`
		WageTypeID *int64 `bun:"wage_type_id"`
	}
	if err := db.NewRaw(`SELECT id, name, is_active, wage_type_id FROM active.staff_absence_types
		WHERE tenant_id = ? ORDER BY lower(name), id`, tenantID).Scan(ctx, &absenceTypes); err != nil {
		return workforce.PayrollRulesState{}, fmt.Errorf("list payroll absence types: %w", err)
	}
	state.AbsenceTypes = make([]workforce.PayrollAbsenceTypeWageType, 0, len(absenceTypes))
	for _, absenceType := range absenceTypes {
		state.AbsenceTypes = append(state.AbsenceTypes, workforce.PayrollAbsenceTypeWageType{
			AbsenceTypeID: absenceType.ID, Name: absenceType.Name, IsActive: absenceType.IsActive, WageTypeID: absenceType.WageTypeID,
		})
	}
	return state, nil
}

func (s *Store) InsertPayrollWageType(ctx context.Context, value workforce.PayrollWageType) (workforce.PayrollWageType, error) {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return workforce.PayrollWageType{}, err
	}
	row := payrollWageTypeRow{TenantID: tenantID, Name: value.Name, Number: value.Number, Unit: value.Unit}
	if err := db.NewInsert().Model(&row).ModelTableExpr("active.payroll_wage_types").
		ExcludeColumn("id", "archived_at").Returning("id").Scan(ctx, &row.ID); err != nil {
		return workforce.PayrollWageType{}, payrollWageTypeWriteError("insert payroll wage type", err)
	}
	return row.toContract(), nil
}

// UpdatePayrollWageType writes name, number and unit of an active wage type;
// archive sets archived_at. It reports false when the school has no such
// active wage type.
func (s *Store) UpdatePayrollWageType(ctx context.Context, value workforce.PayrollWageType) (bool, error) {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewUpdate().Table("active.payroll_wage_types").
		Set("name = ?", value.Name).Set("number = ?", value.Number).Set("unit = ?", value.Unit).
		Where("tenant_id = ? AND id = ? AND archived_at IS NULL", tenantID, value.ID)
	if value.Archived {
		query = query.Set("archived_at = now()")
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return false, payrollWageTypeWriteError("update payroll wage type", err)
	}
	return affectedOne(result)
}

func payrollWageTypeWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, payrollWageTypeNameIndex) {
		return fmt.Errorf("%s: %w", operation, workforce.ErrPayrollWageTypeNameTaken)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func (s *Store) InsertPayrollSurchargeRule(ctx context.Context, value workforce.PayrollSurchargeRule) (workforce.PayrollSurchargeRule, error) {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return workforce.PayrollSurchargeRule{}, err
	}
	row := surchargeRuleRow(tenantID, value)
	if err := db.NewInsert().Model(&row).ModelTableExpr("active.payroll_surcharge_rules").
		ExcludeColumn("id", "archived_at").Returning("id").Scan(ctx, &row.ID); err != nil {
		return workforce.PayrollSurchargeRule{}, fmt.Errorf("insert payroll surcharge rule: %w", err)
	}
	return row.toContract(), nil
}

// UpdatePayrollSurchargeRule writes an active rule; Archived archives it. It
// reports false when the school has no such active rule.
func (s *Store) UpdatePayrollSurchargeRule(ctx context.Context, value workforce.PayrollSurchargeRule) (bool, error) {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return false, err
	}
	row := surchargeRuleRow(tenantID, value)
	query := db.NewUpdate().Table("active.payroll_surcharge_rules").
		Set("kind = ?", row.Kind).
		Set("window_start_minute = ?", row.WindowStartMinute).Set("window_end_minute = ?", row.WindowEndMinute).
		Set("rate_type = ?", row.RateType).Set("rate_hundredths = ?", row.RateHundredths).
		Set("wage_type_id = ?", row.WageTypeID).
		Where("tenant_id = ? AND id = ? AND archived_at IS NULL", tenantID, value.ID)
	if value.Archived {
		query = query.Set("archived_at = now()")
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("update payroll surcharge rule: %w", err)
	}
	return affectedOne(result)
}

// SetAbsenceTypeWageType books an absence type onto a wage type, or back
// onto its category with nil. It reports false for an unknown absence type.
func (s *Store) SetAbsenceTypeWageType(ctx context.Context, absenceTypeID int64, wageTypeID *int64) (bool, error) {
	db, tenantID, err := s.payrollRulesDatabase(ctx)
	if err != nil {
		return false, err
	}
	result, err := db.NewUpdate().Table("active.staff_absence_types").
		Set("wage_type_id = ?", wageTypeID).
		Where("tenant_id = ? AND id = ?", tenantID, absenceTypeID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("set absence type wage type: %w", err)
	}
	return affectedOne(result)
}

func affectedOne(result interface{ RowsAffected() (int64, error) }) (bool, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read affected rows: %w", err)
	}
	return affected == 1, nil
}
