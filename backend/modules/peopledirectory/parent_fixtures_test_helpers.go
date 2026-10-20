//go:build testhooks

package peopledirectory

import (
	"context"
	"encoding/json"
	"time"
)

// These are persisted fixture values, not ORM rows or production portal DTOs.
// The owning stores implement the fixture ports over their unchanged methods.
type GuardianLinkFixture struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        json.RawMessage
}

type GuardianPhoneFixture struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	GuardianProfileID int64
	PhoneNumber       string
	PhoneType         string
	Label             *string
	IsPrimary         bool
	Priority          int
}

type FamilyProtectionFixture struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudentID      int64
	Enabled        bool
	Reason         string
	ActorAccountID int64
}

type ParentRequestEventFixture struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudentID      int64
	RequestType    string
	RequestID      int64
	EventType      string
	ActorAccountID *int64
	Version        string
	Payload        json.RawMessage
}

type GuardianProfileFixture struct {
	ID                     int64
	TenantID               int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	AccountID              *int64
	HasAccount             bool
	PreferredContactMethod string
	LanguagePreference     string
	PortalLocale           *string
	Notes                  *string
	PhoneNumbers           []*GuardianPhoneFixture
}

// GuardianLinkFixtureWriter constructs only persisted relationship fields;
// the retained model's non-persisted Student relation is not a write input.
type GuardianLinkFixtureWriter interface {
	CreateGuardianLinkFixture(context.Context, *GuardianLinkFixture) error
}
type GuardianPhoneFixtureWriter interface {
	CreateGuardianPhoneFixture(context.Context, *GuardianPhoneFixture) error
}
type FamilyProtectionFixtureWriter interface {
	CreateFamilyProtectionFixture(context.Context, *FamilyProtectionFixture) error
}
type ParentRequestEventFixtureSource interface {
	ListParentRequestEventFixtures(context.Context, string, int64) ([]*ParentRequestEventFixture, error)
}

// ParentFixtureProtocol exposes the actual retained scalar vocabulary for the
// cross-owner contract test. It neither re-declares nor normalizes the values.
type ParentFixtureProtocol struct {
	ParentRequestTypeExcusedAbsence    string
	ParentRequestEventSubmitted        string
	ParentRequestEventGuardianEdit     string
	ParentRequestEventDecided          string
	ParentRequestEventMarkedDone       string
	ParentRequestEventCorrected        string
	ParentMessageEventRequestCreated   string
	ParentMessageEventRequestStatus    string
	ParentMessageSenderGuardian        string
	ParentMessageSenderStaff           string
	ParentMessageRequestStatusOpen     string
	ParentMessageRequestStatusDone     string
	ParentMessageRequestStatusRejected string
	ParentMessageRequestExcusedAbsence string
	ParentMessageRequestSickAbsence    string
}

// ParentFixtureProtocols is the test-only scalar facet of a composed owner.
type ParentFixtureProtocols interface{ ParentFixtureProtocol() ParentFixtureProtocol }
