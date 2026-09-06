package postgres

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type requestRow struct {
	bun.BaseModel            `bun:"table:enrollment.requests,alias:request"`
	ID                       int64           `bun:"id,pk,autoincrement"`
	TenantID                 int64           `bun:"tenant_id,notnull"`
	CreatedAt                time.Time       `bun:"created_at"`
	UpdatedAt                time.Time       `bun:"updated_at"`
	SchemaID                 *int64          `bun:"schema_id"`
	PhaseID                  int64           `bun:"phase_id,notnull"`
	GuardianFirstName        string          `bun:"guardian_first_name,notnull"`
	GuardianLastName         string          `bun:"guardian_last_name,notnull"`
	GuardianEmail            string          `bun:"guardian_email,notnull"`
	GuardianPhone            *string         `bun:"guardian_phone"`
	GuardianAccountID        *int64          `bun:"guardian_account_id"`
	ConsentFlags             json.RawMessage `bun:"consent_flags,type:jsonb,notnull,default:'{}'"`
	LegalBlocksSnapshot      json.RawMessage `bun:"legal_blocks_snapshot,type:jsonb,notnull,default:'[]'"`
	CustomData               json.RawMessage `bun:"custom_data,type:jsonb,notnull,default:'{}'"`
	SubmissionSource         string          `bun:"submission_source,notnull,default:'public'"`
	SourceMetadata           json.RawMessage `bun:"source_metadata,type:jsonb,notnull,default:'{}'"`
	StatusToken              string          `bun:"status_token,notnull,unique"`
	StatusTokenExpires       *time.Time      `bun:"status_token_expires"`
	SubmittedAt              time.Time       `bun:"submitted_at,notnull,default:current_timestamp"`
	WithdrawnAt              *time.Time      `bun:"withdrawn_at"`
	DecisionNotificationMode *string         `bun:"decision_notification_mode"`
}

func (r requestRow) value() *enrollment.Request {
	return &enrollment.Request{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SchemaID: r.SchemaID, PhaseID: r.PhaseID, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID, ConsentFlags: r.ConsentFlags, LegalBlocksSnapshot: r.LegalBlocksSnapshot, CustomData: r.CustomData, SubmissionSource: r.SubmissionSource, SourceMetadata: r.SourceMetadata, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires, SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode}
}
func requestStorage(r *enrollment.Request) *requestRow {
	return &requestRow{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SchemaID: r.SchemaID, PhaseID: r.PhaseID, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID, ConsentFlags: r.ConsentFlags, LegalBlocksSnapshot: r.LegalBlocksSnapshot, CustomData: r.CustomData, SubmissionSource: r.SubmissionSource, SourceMetadata: r.SourceMetadata, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires, SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode}
}
