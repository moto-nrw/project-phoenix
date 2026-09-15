package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

type privacyConsentRow struct {
	bun.BaseModel `bun:"table:users.privacy_consents,alias:privacy_consent"`

	ID                int64           `bun:"id,pk,autoincrement"`
	TenantID          int64           `bun:"tenant_id"`
	StudentID         int64           `bun:"student_id"`
	CreatedAt         time.Time       `bun:"created_at"`
	UpdatedAt         time.Time       `bun:"updated_at"`
	PolicyVersion     string          `bun:"policy_version"`
	Accepted          bool            `bun:"accepted"`
	AcceptedAt        *time.Time      `bun:"accepted_at"`
	ExpiresAt         *time.Time      `bun:"expires_at"`
	DurationDays      *int            `bun:"duration_days"`
	RenewalRequired   bool            `bun:"renewal_required"`
	DataRetentionDays int             `bun:"data_retention_days"`
	Details           json.RawMessage `bun:"details,type:jsonb"`
}

func (r privacyConsentRow) toPort() ports.PrivacyConsent {
	return ports.PrivacyConsent{
		ID: r.ID, TenantID: r.TenantID, StudentID: r.StudentID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		PolicyVersion: r.PolicyVersion, Accepted: r.Accepted, AcceptedAt: r.AcceptedAt, ExpiresAt: r.ExpiresAt,
		DurationDays: r.DurationDays, RenewalRequired: r.RenewalRequired, DataRetentionDays: r.DataRetentionDays, Details: r.Details,
	}
}

func (s *Store) ListPrivacyConsents(ctx context.Context, studentID int64) ([]ports.PrivacyConsent, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []privacyConsentRow{}
	started := time.Now()
	err = db.NewSelect().Model(&rows).ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".tenant_id = ?`, tenantID).Where(`"privacy_consent".student_id = ?`, studentID).
		OrderExpr(`"privacy_consent".id ASC`).Scan(ctx)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list privacy consents: %w", err)
	}
	result := make([]ports.PrivacyConsent, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toPort())
	}
	return result, stats, nil
}

func (s *Store) RecordPrivacyConsent(ctx context.Context, value *ports.PrivacyConsent) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	row := privacyConsentRow{
		TenantID: tenantID, StudentID: value.StudentID, PolicyVersion: value.PolicyVersion, Accepted: value.Accepted,
		AcceptedAt: value.AcceptedAt, ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays,
		RenewalRequired: value.RenewalRequired, DataRetentionDays: value.DataRetentionDays, Details: value.Details,
	}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.privacy_consents`).
		Column("tenant_id", "student_id", "policy_version", "accepted", "accepted_at", "expires_at", "duration_days", "renewal_required", "data_retention_days", "details").
		Returning("id, created_at, updated_at").Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("record privacy consent: %w", err)
	}
	stats.Rows = 1
	*value = row.toPort()
	return stats, nil
}

func (s *Store) RevisePrivacyConsent(ctx context.Context, value *ports.PrivacyConsent) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	row := privacyConsentRow{ID: value.ID, StudentID: value.StudentID, TenantID: tenantID,
		PolicyVersion: value.PolicyVersion, Accepted: value.Accepted, AcceptedAt: value.AcceptedAt,
		ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays, RenewalRequired: value.RenewalRequired,
		DataRetentionDays: value.DataRetentionDays, Details: value.Details, UpdatedAt: time.Now()}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr("users.privacy_consents AS privacy_consent").
		Column("policy_version", "accepted", "accepted_at", "expires_at", "duration_days", "renewal_required", "data_retention_days", "details", "updated_at").
		Where("privacy_consent.id = ?", row.ID).Where("privacy_consent.student_id = ?", row.StudentID).Where("privacy_consent.tenant_id = ?", tenantID).
		Returning("created_at").Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("revise privacy consent: %w", err)
	}
	stats.Rows = 1
	*value = row.toPort()
	return stats, nil
}
