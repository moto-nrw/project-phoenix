package organizationtenancy

import (
	"context"
	"net"
	"time"
)

// Provisioning is the operator-led tenant provisioning capability (#3253):
// organisations, schools and their first accounts, the school devices and
// persons an operator manages, and the read model of the operator
// dashboard. Every write records an operator audit entry; clientIP is the
// request address stored with it.
type Provisioning interface {
	CreateOrganization(ctx context.Context, input *CreateOrganization, operatorID int64, clientIP net.IP) (*Organization, error)
	ListOrganizations(ctx context.Context) ([]Organization, error)
	UpdateOrganization(ctx context.Context, id int64, changes OrganizationChanges, operatorID int64, clientIP net.IP) (*Organization, error)
	SoftDeleteOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error
	RestoreOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error

	CreateSchool(ctx context.Context, input *CreateSchool, operatorID int64, clientIP net.IP) (*School, error)
	ListSchools(ctx context.Context) ([]*School, error)
	UpdateSchool(ctx context.Context, id int64, changes SchoolChanges, operatorID int64, clientIP net.IP) (*School, error)
	SoftDeleteSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error
	RestoreSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error

	InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, input SchoolAdminInvitationInput) (*SchoolAdminInvitation, error)
	CreateSchoolAccount(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, input SchoolAccountInput) (*CreatedAccount, error)
	ListSystemRoles(ctx context.Context) ([]SystemRole, error)
	ListSchoolAccounts(ctx context.Context, schoolID int64) ([]SchoolAccount, error)
	ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]OrganizationAccount, error)
	ListAllAccounts(ctx context.Context) ([]OrganizationAccount, error)

	ListAllDevices(ctx context.Context) ([]OperatorDevice, error)
	ListSchoolDevices(ctx context.Context, schoolID int64) ([]OperatorDevice, error)
	ListOrganizationDevices(ctx context.Context, organizationID int64) ([]OperatorDevice, error)
	CreateDevice(ctx context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*OperatorDevice, error)
	SetDeviceAPIKey(ctx context.Context, id int64, apiKey *string, operatorID int64, clientIP net.IP) (*OperatorDevice, error)
	GetDeviceTransferStatus(ctx context.Context, id int64) (*DeviceTransferStatus, error)
	TransferDevice(ctx context.Context, id, targetSchoolID, operatorID int64, clientIP net.IP) (*OperatorDevice, error)
	DeleteDevice(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error

	ListSchoolPersons(ctx context.Context, schoolID int64) ([]OperatorPerson, error)
	ListOrganizationPersons(ctx context.Context, organizationID int64) ([]OperatorPerson, error)
	SoftDeletePerson(ctx context.Context, personID int64, operatorID int64, clientIP net.IP) error

	GetProvisioningStats(ctx context.Context) (*ProvisioningStats, error)
	ListOrganizationSummaries(ctx context.Context) ([]*OrganizationSummary, error)
	ListSchoolSummaries(ctx context.Context) ([]*SchoolSummary, error)
	ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*SchoolSummary, error)
	GetSchoolPWAUsage(ctx context.Context, schoolID int64) (*SchoolPWAUsage, error)
	// ListPWAUsage returns every school's per-portal PWA standalone-usage
	// counts within window, for the metrics exporter.
	ListPWAUsage(ctx context.Context, window time.Duration) ([]SchoolPWAUsageRow, error)
}

// OrganizationChanges are the operator-editable fields of an organisation.
type OrganizationChanges struct {
	Name   string
	Slug   string
	Active bool
}

// SchoolChanges are the operator-editable fields of a school.
type SchoolChanges struct {
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	Active         bool
	Hidden         bool
	// ChildQuota changes the Kinderkontingent (#3567) when set. Nil leaves it
	// as it is, so a school update that does not mention it never clears it.
	ChildQuota *ChildQuotaChange
}

// ChildQuotaChange is the operator's Kinderkontingent decision. A nil Quota
// removes the Kinderkontingent.
type ChildQuotaChange struct {
	Quota *ChildQuota
}

// SchoolAdminInvitationInput invites the first administrator of a school.
type SchoolAdminInvitationInput struct {
	Email            string
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
}

// SchoolAccountInput creates a school account directly. RoleID nil selects
// the admin system role; Position maps to the caregiver profile's role.
type SchoolAccountInput struct {
	Email            string
	Password         string
	FirstName        string
	LastName         string
	RoleID           *int64
	Position         string
	CaregiverEnabled bool
}

// SchoolAdminInvitation is a created school-admin invitation. Token is the
// secret accept token; the HTTP layer exposes it only to the seeder.
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

// CreatedAccount is a school account an operator created.
type CreatedAccount struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Email         string     `json:"email"`
	Username      *string    `json:"username,omitempty"`
	Avatar        string     `json:"avatar,omitempty"`
	Active        bool       `json:"active"`
	IsPasswordOTP bool       `json:"is_password_otp"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
}

// SystemRole is a platform role an operator can hand out. IDs stay decimal
// strings in JSON so frontend code never loses int64 precision.
type SystemRole struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}

// SchoolAccount is one account with access to a school.
type SchoolAccount struct {
	AccountID           int64  `json:"account_id"`
	Email               string `json:"email"`
	Active              bool   `json:"active"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	RoleName            string `json:"role_name"`
	PedagogicRole       string `json:"pedagogic_role"`
	Status              string `json:"status"`
	HasAdminRole        bool   `json:"has_admin_role"`
	HasUserRole         bool   `json:"has_user_role"`
	HasCaregiverProfile bool   `json:"has_caregiver_profile"`
	IsActiveCaregiver   bool   `json:"is_active_caregiver"`
}

// OrganizationAccount is a school account with its school for the
// organisation-wide and platform-wide listings.
type OrganizationAccount struct {
	SchoolAccount
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
}

// OperatorDevice is one device in the operator device listings with its
// school and organisation. MaskedAPIKey and IsOnline are derived.
type OperatorDevice struct {
	ID               int64      `json:"id"`
	DeviceID         string     `json:"device_id"`
	DeviceType       string     `json:"device_type"`
	Name             *string    `json:"name,omitempty"`
	Status           string     `json:"status"`
	APIKey           *string    `json:"api_key,omitempty"`
	MaskedAPIKey     string     `json:"masked_api_key"`
	LastSeen         *time.Time `json:"last_seen,omitempty"`
	IsOnline         bool       `json:"is_online"`
	SchoolID         int64      `json:"school_id"`
	SchoolName       string     `json:"school_name"`
	OrganizationID   int64      `json:"organization_id"`
	OrganizationName string     `json:"organization_name"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// DeviceTransferSession describes the open kiosk session blocking a transfer.
type DeviceTransferSession struct {
	ID           int64     `json:"id"`
	StartedAt    time.Time `json:"started_at"`
	ActivityName *string   `json:"activity_name,omitempty"`
	RoomName     *string   `json:"room_name,omitempty"`
}

// DeviceTransferStatus is the operator-facing transfer preflight result.
type DeviceTransferStatus struct {
	CanTransfer   bool                   `json:"can_transfer"`
	IsOnline      bool                   `json:"is_online"`
	IsProtected   bool                   `json:"is_protected"`
	LastSeen      *time.Time             `json:"last_seen,omitempty"`
	ActiveSession *DeviceTransferSession `json:"active_session,omitempty"`
}

// OperatorPerson is one person with school and organisation context.
type OperatorPerson struct {
	ID               int64     `json:"id"`
	FirstName        string    `json:"first_name"`
	LastName         string    `json:"last_name"`
	HasAccount       bool      `json:"has_account"`
	AccountEmail     *string   `json:"account_email,omitempty"`
	HasRFIDCard      bool      `json:"has_rfid_card"`
	IsStaff          bool      `json:"is_staff"`
	IsStudent        bool      `json:"is_student"`
	SchoolID         int64     `json:"school_id"`
	SchoolName       string    `json:"school_name"`
	OrganizationID   int64     `json:"organization_id"`
	OrganizationName string    `json:"organization_name"`
	CreatedAt        time.Time `json:"created_at"`
}

// ProvisioningStats are the platform-wide counts of the operator overview.
// All counts exclude soft-deleted rows. KontenCount counts an account active
// in several schools once, so it is not the sum of the school counts.
type ProvisioningStats struct {
	TraegerCount int `json:"traeger_count"`
	SchulenCount int `json:"schulen_count"`
	KontenCount  int `json:"konten_count"`
	GeraeteCount int `json:"geraete_count"`
}

// OrganizationSummary is an organisation with the counts of its non-deleted
// schools, accounts, devices and persons. KontenCount counts an account once
// per organisation.
type OrganizationSummary struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Slug          string     `json:"slug"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	Settings      string     `json:"settings,omitempty"`
	SchulenCount  int        `json:"schulen_count"`
	KontenCount   int        `json:"konten_count"`
	GeraeteCount  int        `json:"geraete_count"`
	PersonenCount int        `json:"personen_count"`
}

// SchoolSummary is a school with its organisation's name and the counts of
// its accounts, devices and persons.
type SchoolSummary struct {
	ID               int64      `json:"id"`
	OrganizationID   int64      `json:"organization_id"`
	OrganizationName string     `json:"organization_name"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	Subdomain        string     `json:"subdomain"`
	Active           bool       `json:"active"`
	Hidden           bool       `json:"hidden"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	Address          string     `json:"address,omitempty"`
	City             string     `json:"city,omitempty"`
	Zip              string     `json:"zip,omitempty"`
	Phone            string     `json:"phone,omitempty"`
	Email            string     `json:"email,omitempty"`
	Settings         string     `json:"settings,omitempty"`
	KontenCount      int        `json:"konten_count"`
	GeraeteCount     int        `json:"geraete_count"`
	PersonenCount    int        `json:"personen_count"`
	// ChildQuotaBundles and ChildQuotaBundleSize are the Kinderkontingent
	// (#3567); bundles are null when the school has none. ChildQuotaCount is
	// the Kontingentzahl it is checked against (#3568).
	ChildQuotaBundles    *int `json:"child_quota_bundles"`
	ChildQuotaBundleSize int  `json:"child_quota_bundle_size"`
	ChildQuotaCount      int  `json:"child_quota_count"`
}

// PWAPortalUsage is one portal's slice of a school's PWA standalone usage.
type PWAPortalUsage struct {
	StandaloneUsers int `json:"standalone_users"`
	EligibleUsers   int `json:"eligible_users"`
}

// SchoolPWAUsage is how many of a school's staff and parent accounts used
// the app in standalone display mode within the window (#2189). It is not
// an install count; the browser offers no honest install signal.
type SchoolPWAUsage struct {
	WindowDays int            `json:"window_days"`
	Staff      PWAPortalUsage `json:"staff"`
	Parent     PWAPortalUsage `json:"parent"`
}

// SchoolPWAUsageRow is one (school, portal) bucket of PWA standalone-usage
// counts. StandaloneUsers never exceeds EligibleUsers.
type SchoolPWAUsageRow struct {
	TenantID        int64
	Portal          string
	StandaloneUsers int
	EligibleUsers   int
}
