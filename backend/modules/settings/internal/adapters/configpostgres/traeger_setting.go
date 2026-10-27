package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/uptrace/bun"
)

const (
	tableTraegerSettingValues      = "config.traeger_setting_values"
	tableTraegerSettingValuesAlias = `config.traeger_setting_values AS "traeger_setting_value"`
	tableTraegerSettingAudit       = "config.traeger_setting_audit"
)

// TraegerSettingRepository implements config.TraegerSettingRepository. Every
// query names the Träger or the school it reads for, in addition to the
// row-level security of the transaction it runs in (ADR 0086 section 5). It
// never reads platform.schools: which Träger a school has is Organisation &
// Tenancy's to say, through its query or the school policy of the table.
type TraegerSettingRepository struct {
	runtime Runtime
}

// NewTraegerSettingRepository builds the Träger level store over the caller's
// settings runtime.
func NewTraegerSettingRepository(runtime Runtime) config.TraegerSettingRepository {
	return &TraegerSettingRepository{runtime: runtime}
}

// FindForSchool reads the values of the school's Träger in the school's
// tenant transaction. The read-only school policy of the table selects the
// school's Träger; the predicate on app.current_tenant_id makes any other
// transaction, an administrative one included, read nothing instead of the
// values of every Träger.
func (r *TraegerSettingRepository) FindForSchool(ctx context.Context, tenantID int64, keys []string) ([]*config.TraegerSettingValue, error) {
	if tenantID <= 0 || len(keys) == 0 {
		return nil, nil
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	var values []*config.TraegerSettingValue
	err = db.NewSelect().
		Model(&values).
		ModelTableExpr(tableTraegerSettingValuesAlias).
		Where(`NULLIF(current_setting('app.current_tenant_id', true), '')::bigint = ?`, tenantID).
		Where(`"traeger_setting_value".setting_key IN (?)`, bun.List(keys)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find Träger setting values for school: %w", err)
	}
	return values, nil
}

// FindByOrganizations reads the values of several Träger. The caller provides
// the administrative transaction of the batch read (ResolveManyForTenants).
func (r *TraegerSettingRepository) FindByOrganizations(ctx context.Context, organizationIDs []int64, keys []string) ([]*config.TraegerSettingValue, error) {
	if len(organizationIDs) == 0 || len(keys) == 0 {
		return nil, nil
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	var values []*config.TraegerSettingValue
	err = db.NewSelect().
		Model(&values).
		ModelTableExpr(tableTraegerSettingValuesAlias).
		Where(`"traeger_setting_value".organization_id IN (?)`, bun.List(organizationIDs)).
		Where(`"traeger_setting_value".setting_key IN (?)`, bun.List(keys)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find Träger setting values of several Träger: %w", err)
	}
	return values, nil
}

// FindByOrganization reads the Träger's own values in its Träger transaction.
func (r *TraegerSettingRepository) FindByOrganization(ctx context.Context, organizationID int64, keys []string) ([]*config.TraegerSettingValue, error) {
	if organizationID <= 0 || len(keys) == 0 {
		return nil, nil
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	var values []*config.TraegerSettingValue
	err = db.NewSelect().
		Model(&values).
		ModelTableExpr(tableTraegerSettingValuesAlias).
		Where(`"traeger_setting_value".organization_id = ?`, organizationID).
		Where(`"traeger_setting_value".setting_key IN (?)`, bun.List(keys)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find Träger setting values: %w", err)
	}
	return values, nil
}

// LockByOrganizationAndKey reads the Träger's value of key FOR UPDATE, so the
// old value in the change log is the one the change replaced.
func (r *TraegerSettingRepository) LockByOrganizationAndKey(ctx context.Context, organizationID int64, key string) (*config.TraegerSettingValue, error) {
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	value := new(config.TraegerSettingValue)
	err = db.NewSelect().
		Model(value).
		ModelTableExpr(tableTraegerSettingValuesAlias).
		Where(`"traeger_setting_value".organization_id = ?`, organizationID).
		Where(`"traeger_setting_value".setting_key = ?`, key).
		For("UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock Träger setting value: %w", err)
	}
	return value, nil
}

// Upsert inserts or replaces the Träger's value of a key.
func (r *TraegerSettingRepository) Upsert(ctx context.Context, value *config.TraegerSettingValue) error {
	if value == nil || value.OrganizationID <= 0 || value.SettingKey == "" || len(value.Value) == 0 {
		return fmt.Errorf("upsert Träger setting value: Träger, key and value are required")
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewInsert().
		Model(value).
		ModelTableExpr(tableTraegerSettingValues).
		On("CONFLICT (organization_id, setting_key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert Träger setting value: %w", err)
	}
	return nil
}

// Delete removes the Träger's value of a key; its schools fall back to the
// registry default or keep their own override.
func (r *TraegerSettingRepository) Delete(ctx context.Context, organizationID int64, key string) error {
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().
		TableExpr(tableTraegerSettingValues).
		Where("organization_id = ?", organizationID).
		Where("setting_key = ?", key).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete Träger setting value: %w", err)
	}
	return nil
}

// RecordChange appends one change of a Träger value and drops the Träger's
// changes older than the retention in the same transaction.
func (r *TraegerSettingRepository) RecordChange(ctx context.Context, entry *config.TraegerSettingAuditEntry, retainedSince time.Time) error {
	if entry == nil || entry.OrganizationID <= 0 || entry.SettingKey == "" || (entry.Action != "set" && entry.Action != "reset") {
		return fmt.Errorf("record Träger setting change: Träger, key and action set or reset are required")
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return err
	}
	if _, err = db.NewInsert().Model(entry).ModelTableExpr(tableTraegerSettingAudit).Exec(ctx); err != nil {
		return fmt.Errorf("record Träger setting change: %w", err)
	}
	_, err = db.NewDelete().
		TableExpr(tableTraegerSettingAudit).
		Where("organization_id = ?", entry.OrganizationID).
		Where("changed_at < ?", retainedSince).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete expired Träger setting changes: %w", err)
	}
	return nil
}
