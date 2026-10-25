package care

import (
	"encoding/json"
	"time"
)

type GuardianRelationship struct {
	ID                 int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	TenantID           int64
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	// PickupWeekdays: plan weekdays the pickup permission holds on, nil for
	// every day (#4780).
	PickupWeekdays    []int
	EmergencyPriority int
	IsPayer           bool
	Permissions       json.RawMessage
	Student           *PortalStudent
}
type GuardianLinkedChild struct {
	GuardianProfileID int64
	StudentID         int64
	FirstName         string
	LastName          string
}
