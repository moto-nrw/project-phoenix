package test

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// The Device Fleet rows the fixtures write and the suites read back. The
// owner keeps its row mapping in models/iot, which the shared fixtures may
// not import (#2748), so test support maps the table itself, like the School
// Structure rows.

// The device values the fixtures write.
const (
	DeviceStatusActive = "active"
	DeviceTypeVirtual  = "virtual"
	WebManualDeviceID  = "WEB-MANUAL-001"
)

// Device is one iot.devices row.
type Device struct {
	bun.BaseModel         `bun:"table:iot.devices,alias:device"`
	ID                    int64      `bun:"id,pk,autoincrement"`
	CreatedAt             time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID              int64      `bun:"tenant_id,notnull"`
	DeviceID              string     `bun:"device_id,notnull"`
	DeviceType            string     `bun:"device_type,notnull"`
	Name                  *string    `bun:"name"`
	Status                string     `bun:"status,notnull,default:'active'"`
	APIKey                *string    `bun:"-"`
	APIKeyHash            *string    `bun:"api_key_hash,unique"`
	MaskedAPIKey          string     `bun:"masked_api_key"`
	LastSeen              *time.Time `bun:"last_seen"`
	RegisteredByID        *int64     `bun:"registered_by_id"`
	RoomID                *int64     `bun:"room_id"`
	ArchivedAt            *time.Time `bun:"archived_at"`
	TransferredToDeviceID *int64     `bun:"transferred_to_device_id"`
}

// BeforeInsert persists only a fingerprint of the transient fixture key.
func (*Device) BeforeInsert(_ context.Context, query *bun.InsertQuery) error {
	d, ok := query.GetModel().Value().(*Device)
	if ok && d.APIKey != nil {
		query.Value("api_key_hash", "encode(sha256(convert_to(?, 'UTF8')), 'hex')", *d.APIKey)
		query.Value("masked_api_key", "?", "...")
	}
	return nil
}
