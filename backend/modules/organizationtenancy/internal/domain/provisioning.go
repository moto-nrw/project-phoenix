package domain

import (
	"errors"
	"net"
	"time"
)

// Operator audit vocabulary the provisioning flows record (#3253). The
// values are the stored platform.operator_audit_log strings.
const (
	AuditActionView         = "view"
	AuditActionCreate       = "create"
	AuditActionUpdate       = "update"
	AuditActionDelete       = "delete"
	AuditActionRotateAPIKey = "rotate_api_key"
	AuditActionTransfer     = "transfer"
	AuditActionSoftDelete   = "soft_delete"
	AuditActionRestore      = "restore"
	AuditActionGrant        = "grant"
	// The Träger erasure order (#4657); the erasure CLI records "erase"
	// itself when it deletes the Träger root (migration 1.15.533).
	AuditActionRecordErasureOrder   = "record_erasure_order"
	AuditActionWithdrawErasureOrder = "withdraw_erasure_order"

	AuditResourceOrganization  = "organization"
	AuditResourceSchool        = "school"
	AuditResourceInvitation    = "invitation"
	AuditResourceDevice        = "device"
	AuditResourceAccount       = "account"
	AuditResourcePerson        = "person"
	AuditResourceAnnouncement  = "announcement"
	AuditResourceTagScan       = "unregistered_tag_scan"
	AuditResourceTraegerMember = "traeger_member"
	// AuditResourceSampleChildren is the sample children of a test Träger
	// (#4658); the resource ID is the Träger.
	AuditResourceSampleChildren = "sample_children"
)

// Device vocabulary shared with Device Fleet.
const (
	WebManualDeviceID    = "WEB-MANUAL-001"
	DeviceTypeVirtual    = "virtual"
	DeviceStatusActive   = "active"
	DeviceStatusInactive = "inactive"
)

// Port errors the provisioning adapters translate their owners' failures to.
var (
	// ErrDeviceIDTaken reports a device_id already registered at the school.
	ErrDeviceIDTaken = errors.New("device id already exists for this school")
	// ErrDeviceAPIKeyTaken reports an API key another device already holds.
	ErrDeviceAPIKeyTaken = errors.New("device api key already in use")
	// ErrDeviceReferenced reports a device attendance or session rows still
	// reference.
	ErrDeviceReferenced = errors.New("device is still referenced")
	// ErrInvalidSchoolIdentity reports names or a person the school identity
	// chain cannot be provisioned from.
	ErrInvalidSchoolIdentity = errors.New("invalid school identity")
)

// OperatorAuditEntry is one operator action for the platform audit ledger.
// Changes is the JSON change summary, nil when there is none.
type OperatorAuditEntry struct {
	OperatorID   int64
	Action       string
	ResourceType string
	ResourceID   *int64
	ClientIP     net.IP
	Changes      []byte
}

// Role is a role as the provisioning flows see it. Identity & Access
// classifies it when it hands the role out.
type Role struct {
	ID       int64
	Name     string
	IsSystem bool
	TenantID *int64
	BaseRole *string
	// Lehrkraft marks the class-scoped read-only school role that must not
	// be combined with the caregiver capability (#1772).
	Lehrkraft bool
	// CaregiverPermissions reports whether the role already carries the
	// caregiver permissions.
	CaregiverPermissions bool
}

// SchoolAdminInvitationRequest invites an administrator at one school.
type SchoolAdminInvitationRequest struct {
	TenantID         int64
	RoleID           int64
	Email            string
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
}

// SchoolAdminInvitation is a created invitation.
type SchoolAdminInvitation struct {
	ID               int64
	Email            string
	RoleID           int64
	RoleName         string
	Token            string
	ExpiresAt        time.Time
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	CreatedBy        *int64
	CreatorEmail     string
	EmailSentAt      *time.Time
	EmailError       *string
	EmailRetryCount  int
}

// SchoolAccountRegistration creates an account at one school with one role.
type SchoolAccountRegistration struct {
	TenantID int64
	Email    string
	Username string
	Password string
	RoleID   int64
}

// CreatedAccount is a registered account.
type CreatedAccount struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Email         string
	Username      *string
	Avatar        string
	Active        bool
	IsPasswordOTP bool
	LastLogin     *time.Time
}

// SchoolIdentityRequest provisions the person, staff and caregiver chain of
// an account at one school.
type SchoolIdentityRequest struct {
	AccountID        int64
	TenantID         int64
	Role             Role
	FirstName        string
	LastName         string
	Position         string
	CaregiverUpgrade bool
}

// SchoolAccount is one account with access to a school.
type SchoolAccount struct {
	AccountID           int64
	Email               string
	Active              bool
	FirstName           string
	LastName            string
	RoleName            string
	PedagogicRole       string
	Status              string
	HasAdminRole        bool
	HasUserRole         bool
	HasCaregiverProfile bool
	IsActiveCaregiver   bool
}

// OrganizationAccount is a school account with its school.
type OrganizationAccount struct {
	SchoolAccount
	SchoolID   int64
	SchoolName string
}

// Device is one fleet device with every column provisioning writes.
type Device struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeviceID              string
	DeviceType            string
	Name                  *string
	Status                string
	APIKey                *string
	APIKeyHash            *string
	MaskedAPIKey          string
	LastSeen              *time.Time
	RegisteredByID        *int64
	RoomID                *int64
	ArchivedAt            *time.Time
	TransferredToDeviceID *int64
}

// NewDevice registers a device at one school.
type NewDevice struct {
	TenantID     int64
	DeviceID     string
	DeviceType   string
	Name         *string
	Status       string
	APIKey       *string
	APIKeyHash   *string
	MaskedAPIKey string
}

// DeviceSession is the open kiosk session of a device.
type DeviceSession struct {
	ID           int64
	StartedAt    time.Time
	ActivityName *string
	RoomName     *string
}

// Person is the part of a person the soft delete reads. FormerStaff is true
// when the person holds retired staff memberships and no live one.
type Person struct {
	ID          int64
	TenantID    int64
	AccountID   *int64
	HasRFIDCard bool
	FormerStaff bool
}

// PersonListing is one person of the operator person listings.
type PersonListing struct {
	ID           int64
	TenantID     int64
	FirstName    string
	LastName     string
	HasAccount   bool
	AccountEmail *string
	HasRFIDCard  bool
	IsStaff      bool
	IsStudent    bool
	CreatedAt    time.Time
}

// StaffMember identifies the staff record of a person.
type StaffMember struct {
	ID       int64
	TenantID int64
}

// ActivityCategory is one default activity category a new school receives.
type ActivityCategory struct {
	Name        string
	Description string
	Color       string
}
