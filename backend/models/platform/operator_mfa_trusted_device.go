package platform

import (
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// OperatorMFATrustedDevice persists the server-side record of a trusted-
// device cookie issued to a moto-Operator. Mirror of auth.MFATrustedDevice
// against platform.operator_mfa_trusted_devices.
type OperatorMFATrustedDevice struct {
	base.Model `bun:"schema:platform,table:operator_mfa_trusted_devices"`
	OperatorID int64      `bun:"operator_id,notnull" json:"operator_id"`
	TokenHash  string     `bun:"token_hash,notnull" json:"-"`
	UserAgent  *string    `bun:"user_agent" json:"user_agent,omitempty"`
	IPAddress  net.IP     `bun:"ip_address,type:inet,nullzero" json:"ip_address,omitempty"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	LastUsedAt *time.Time `bun:"last_used_at" json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `bun:"revoked_at" json:"revoked_at,omitempty"`
}

func (d *OperatorMFATrustedDevice) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_trusted_devices AS "operator_mfa_trusted_device"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_trusted_devices AS "operator_mfa_trusted_device"`)
	}
	return nil
}

func (d *OperatorMFATrustedDevice) TableName() string {
	return "platform.operator_mfa_trusted_devices"
}

// IsExpired returns true once the device's trust window has passed.
func (d *OperatorMFATrustedDevice) IsExpired() bool { return time.Now().After(d.ExpiresAt) }

// IsRevoked returns true once the device was explicitly revoked.
func (d *OperatorMFATrustedDevice) IsRevoked() bool { return d.RevokedAt != nil }

// IsActive is the convenience predicate for the trusted-device check on login.
func (d *OperatorMFATrustedDevice) IsActive() bool { return !d.IsExpired() && !d.IsRevoked() }

func (d *OperatorMFATrustedDevice) GetID() interface{}      { return d.ID }
func (d *OperatorMFATrustedDevice) GetCreatedAt() time.Time { return d.CreatedAt }
func (d *OperatorMFATrustedDevice) GetUpdatedAt() time.Time { return d.UpdatedAt }
