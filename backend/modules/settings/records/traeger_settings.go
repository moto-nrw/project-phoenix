package records

import (
	"context"
	"encoding/json"
	"time"
)

// TraegerSettingValue is the value a Träger set for all its schools (ADR
// 0086).
type TraegerSettingValue struct {
	ID             int64           `bun:"id,pk,autoincrement" json:"id"`
	OrganizationID int64           `bun:"organization_id,notnull" json:"organization_id"`
	SettingKey     string          `bun:"setting_key,notnull" json:"setting_key"`
	Value          json.RawMessage `bun:"value,type:jsonb,notnull" json:"value"`
	UpdatedBy      *int64          `bun:"updated_by" json:"updated_by,omitempty"`
	CreatedAt      time.Time       `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt      time.Time       `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

// TraegerSettingAuditEntry records one change of a Träger value. Append-only.
type TraegerSettingAuditEntry struct {
	ID             int64           `bun:"id,pk,autoincrement" json:"id"`
	OrganizationID int64           `bun:"organization_id,notnull" json:"organization_id"`
	SettingKey     string          `bun:"setting_key,notnull" json:"setting_key"`
	OldValue       json.RawMessage `bun:"old_value,type:jsonb,nullzero" json:"old_value,omitempty"`
	NewValue       json.RawMessage `bun:"new_value,type:jsonb,nullzero" json:"new_value,omitempty"`
	Action         string          `bun:"action,notnull" json:"action"` // set, reset
	ChangedBy      *int64          `bun:"changed_by" json:"changed_by,omitempty"`
	ChangedAt      time.Time       `bun:"changed_at,notnull,default:current_timestamp" json:"changed_at"`
}

// TraegerSettingRepository is the Settings Platform's store of the Träger
// level. The Träger methods run in the Träger transaction of the
// organization; the school reads run in the school's tenant transaction or,
// for several schools, in an administrative one.
type TraegerSettingRepository interface {
	// FindForSchool returns the values the school's Träger set for the keys.
	// It answers only inside that school's tenant transaction, whose
	// read-only policy admits exactly the rows of the school's Träger.
	FindForSchool(ctx context.Context, tenantID int64, keys []string) ([]*TraegerSettingValue, error)
	// FindByOrganizations returns the values of several Träger for the keys;
	// the caller provides the administrative transaction of a batch read.
	FindByOrganizations(ctx context.Context, organizationIDs []int64, keys []string) ([]*TraegerSettingValue, error)
	// FindByOrganization returns the Träger's values for the keys.
	FindByOrganization(ctx context.Context, organizationID int64, keys []string) ([]*TraegerSettingValue, error)
	// LockByOrganizationAndKey returns the Träger's value of key under a row
	// lock, or nil when it has none.
	LockByOrganizationAndKey(ctx context.Context, organizationID int64, key string) (*TraegerSettingValue, error)
	Upsert(ctx context.Context, value *TraegerSettingValue) error
	Delete(ctx context.Context, organizationID int64, key string) error
	// RecordChange appends a change to the Träger's change log and drops
	// its changes made before retainedSince, in the caller's transaction.
	RecordChange(ctx context.Context, entry *TraegerSettingAuditEntry, retainedSince time.Time) error
}
