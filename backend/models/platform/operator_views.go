package platform

import (
	"context"
	"time"
)

// ProvisioningStats holds platform-wide aggregate counts for the operator
// overview KPI strip. All counts exclude soft-deleted rows.
//
// KontenCount is DISTINCT account_id across all active account-tenant
// memberships, so an account active in N schools counts once at the platform
// level. That account still contributes one row to each of those N schools'
// own KontenCount, so the platform total will not equal the sum of school
// totals (and an organization's total will not equal the sum of its schools'
// totals either). Do not derive parent counts by summing children.
type ProvisioningStats struct {
	TraegerCount int `bun:"traeger_count" json:"traeger_count"`
	SchulenCount int `bun:"schulen_count" json:"schulen_count"`
	KontenCount  int `bun:"konten_count" json:"konten_count"`
	GeraeteCount int `bun:"geraete_count" json:"geraete_count"`
}

// OrganizationSummary is an organization row enriched with aggregate counts
// scoped to that organization. Counts reflect non-deleted child entities only.
//
// KontenCount uses DISTINCT account_id within the org's schools, so an account
// active in multiple schools of the same org counts once for the org row but
// once per school in SchoolSummary. Do not sum SchoolSummary.KontenCount to
// reconstruct this value.
type OrganizationSummary struct {
	ID            int64      `bun:"id" json:"id"`
	Name          string     `bun:"name" json:"name"`
	Slug          string     `bun:"slug" json:"slug"`
	Active        bool       `bun:"active" json:"active"`
	CreatedAt     time.Time  `bun:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Settings      string     `bun:"settings" json:"settings,omitempty"`
	SchulenCount  int        `bun:"schulen_count" json:"schulen_count"`
	KontenCount   int        `bun:"konten_count" json:"konten_count"`
	GeraeteCount  int        `bun:"geraete_count" json:"geraete_count"`
	PersonenCount int        `bun:"personen_count" json:"personen_count"`
}

// SchoolSummary is a school row enriched with aggregate counts scoped to that
// school. Counts reflect non-deleted child entities only. OrganizationName is
// denormalized from the parent for display in the global Schulen list.
//
// KontenCount is DISTINCT account_id within this school. The same account may
// also appear in sibling schools, so summing KontenCount across schools
// double-counts shared accounts and will exceed the parent organization's or
// platform's KontenCount.
type SchoolSummary struct {
	ID               int64      `bun:"id" json:"id"`
	OrganizationID   int64      `bun:"organization_id" json:"organization_id"`
	OrganizationName string     `bun:"organization_name" json:"organization_name"`
	Name             string     `bun:"name" json:"name"`
	Slug             string     `bun:"slug" json:"slug"`
	Subdomain        string     `bun:"subdomain" json:"subdomain"`
	Active           bool       `bun:"active" json:"active"`
	Hidden           bool       `bun:"hidden" json:"hidden"`
	CreatedAt        time.Time  `bun:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `bun:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Address          string     `bun:"address" json:"address,omitempty"`
	City             string     `bun:"city" json:"city,omitempty"`
	Zip              string     `bun:"zip" json:"zip,omitempty"`
	Phone            string     `bun:"phone" json:"phone,omitempty"`
	Email            string     `bun:"email" json:"email,omitempty"`
	Settings         string     `bun:"settings" json:"settings,omitempty"`
	KontenCount      int        `bun:"konten_count" json:"konten_count"`
	GeraeteCount     int        `bun:"geraete_count" json:"geraete_count"`
	PersonenCount    int        `bun:"personen_count" json:"personen_count"`
}

// OperatorPersonInfo holds person information with school/org context for
// operator views.
type OperatorPersonInfo struct {
	ID               int64     `bun:"id" json:"id"`
	FirstName        string    `bun:"first_name" json:"first_name"`
	LastName         string    `bun:"last_name" json:"last_name"`
	HasAccount       bool      `bun:"has_account" json:"has_account"`
	AccountEmail     *string   `bun:"account_email" json:"account_email,omitempty"`
	HasRFIDCard      bool      `bun:"has_rfid_card" json:"has_rfid_card"`
	IsStaff          bool      `bun:"is_staff" json:"is_staff"`
	IsStudent        bool      `bun:"is_student" json:"is_student"`
	SchoolID         int64     `bun:"school_id" json:"school_id"`
	SchoolName       string    `bun:"school_name" json:"school_name"`
	OrganizationID   int64     `bun:"organization_id" json:"organization_id"`
	OrganizationName string    `bun:"organization_name" json:"organization_name"`
	CreatedAt        time.Time `bun:"created_at" json:"created_at"`
}

// SchoolPWAUsageRow is one (school, portal) bucket of PWA standalone-usage
// counts (#2189). EligibleUsers is the number of accounts with an active
// mapping in that school matching the portal's role predicate (staff = any
// non-guardian role, parent = guardian role); StandaloneUsers is how many of
// those used the app in standalone display mode within the query window.
// StandaloneUsers <= EligibleUsers always holds because both sides use the
// same predicate.
type SchoolPWAUsageRow struct {
	TenantID        int64  `bun:"tenant_id" json:"tenant_id"`
	Portal          string `bun:"portal" json:"portal"`
	StandaloneUsers int    `bun:"standalone_users" json:"standalone_users"`
	EligibleUsers   int    `bun:"eligible_users" json:"eligible_users"`
}

// OperatorSummariesRepository defines read-only aggregate queries that power
// the operator dashboard's drill-in views. Each method runs a single CTE-based
// query that participates in any transaction stored on the context via
// modelBase.ContextWithTx, so it composes with TenantTx/AdminTx middleware.
type OperatorSummariesRepository interface {
	// ListDeviceRows returns the operator device listing (optionally
	// filtered by device row, school, or organization), excluding devices
	// of soft-deleted schools/organizations.
	ListDeviceRows(ctx context.Context, filter OperatorDeviceFilter) ([]OperatorDeviceRow, error)

	// Stats returns platform-wide counts (Träger, Schulen, Konten, Geräte).
	// Konten count is DISTINCT account_id across all active account-tenant
	// memberships; the same account active in multiple schools counts once.
	Stats(ctx context.Context) (*ProvisioningStats, error)

	// OrganizationSummaries returns every organization (including soft-deleted)
	// with per-row aggregate counts scoped to that organization.
	OrganizationSummaries(ctx context.Context) ([]*OrganizationSummary, error)

	// SchoolSummaries returns every school globally with per-row counts and the
	// parent organization's name denormalized onto each row.
	SchoolSummaries(ctx context.Context) ([]*SchoolSummary, error)

	// SchoolSummariesByOrganization returns schools (including soft-deleted)
	// belonging to a single organization with per-row counts. The caller is
	// responsible for verifying the organization exists.
	SchoolSummariesByOrganization(ctx context.Context, organizationID int64) ([]*SchoolSummary, error)

	// PersonsBySchool returns non-deleted persons in a single school with
	// school/org context on each row.
	PersonsBySchool(ctx context.Context, schoolID int64) ([]OperatorPersonInfo, error)

	// PersonsByOrganization returns non-deleted persons across every school
	// belonging to the given organization with school/org context on each row.
	PersonsByOrganization(ctx context.Context, organizationID int64) ([]OperatorPersonInfo, error)

	// PWAUsage returns per-school, per-portal PWA standalone-usage counts
	// within the given window. tenantID > 0 limits to one school; 0 returns
	// every school. Callers must run inside an admin transaction.
	PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]SchoolPWAUsageRow, error)
}

// OperatorDeviceRow is one device in the operator device listings, joined
// with its school and organization. MaskedAPIKey and IsOnline are derived
// presentation fields the service computes after the read.
type OperatorDeviceRow struct {
	ID               int64      `bun:"id" json:"id"`
	DeviceID         string     `bun:"device_id" json:"device_id"`
	DeviceType       string     `bun:"device_type" json:"device_type"`
	Name             *string    `bun:"name" json:"name,omitempty"`
	Status           string     `bun:"status" json:"status"`
	APIKey           *string    `bun:"api_key" json:"api_key,omitempty"`
	MaskedAPIKey     string     `bun:"-" json:"masked_api_key"`
	LastSeen         *time.Time `bun:"last_seen" json:"last_seen,omitempty"`
	IsOnline         bool       `bun:"-" json:"is_online"`
	SchoolID         int64      `bun:"school_id" json:"school_id"`
	SchoolName       string     `bun:"school_name" json:"school_name"`
	OrganizationID   int64      `bun:"organization_id" json:"organization_id"`
	OrganizationName string     `bun:"organization_name" json:"organization_name"`
	CreatedAt        time.Time  `bun:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `bun:"updated_at" json:"updated_at"`
}

// OperatorDeviceFilter narrows the operator device listing. At most one
// field is set; the zero value lists every device.
type OperatorDeviceFilter struct {
	DeviceRowID    *int64
	SchoolID       *int64
	OrganizationID *int64
}
