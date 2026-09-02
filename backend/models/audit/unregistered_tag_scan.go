package audit

import "time"

// UnregisteredTagScan records an RFID tag UID that was scanned by an IoT
// device but was not assigned to any person at scan time.
type UnregisteredTagScan struct {
	Model
	TenantModel
	TagUID               string     `bun:"tag_uid,notnull" json:"tag_uid"`
	DeviceID             *int64     `bun:"device_id" json:"device_id,omitempty"`
	ScannedAt            time.Time  `bun:"scanned_at,notnull,default:now()" json:"scanned_at"`
	ResolvedAt           *time.Time `bun:"resolved_at" json:"resolved_at,omitempty"`
	ResolvedByOperatorID *int64     `bun:"resolved_by_operator_id" json:"resolved_by_operator_id,omitempty"`
	ResolutionNote       *string    `bun:"resolution_note" json:"resolution_note,omitempty"`

	SchoolID         int64   `bun:"school_id,scanonly" json:"school_id,omitempty"`
	SchoolName       string  `bun:"school_name,scanonly" json:"school_name,omitempty"`
	OrganizationID   int64   `bun:"organization_id,scanonly" json:"organization_id,omitempty"`
	OrganizationName string  `bun:"organization_name,scanonly" json:"organization_name,omitempty"`
	DeviceIdentifier *string `bun:"device_identifier,scanonly" json:"device_identifier,omitempty"`
	DeviceName       *string `bun:"device_name,scanonly" json:"device_name,omitempty"`
}

type UnregisteredTagScanFilter struct {
	SchoolID       *int64
	OrganizationID *int64
	SchoolIDs      []int64
	UnresolvedOnly bool
}
