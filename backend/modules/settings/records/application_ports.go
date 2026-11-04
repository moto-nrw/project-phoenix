package records

import "context"

// TenantValueSetHook runs domain-owned side effects in the write transaction.
type TenantValueSetHook func(context.Context, int64, string, any) (func(), error)

// LegalDocumentCleanup prepares post-commit cleanup for a replaced document.
type LegalDocumentCleanup func(context.Context, int64, string, string) (func(), error)

type TenantOperationsRuntime interface {
	WithinTenant(context.Context, int64, func(context.Context) error) error
	AcquireLock(context.Context, string, bool) error
	AfterCommit(context.Context, func())
}

// Runtime supplies infrastructure operations without coupling settings
// application code to the transaction, ORM, or authorization adapters.
type Runtime interface {
	TenantID(context.Context) int64
	HasTransaction(context.Context) bool
	WithinTenant(context.Context, int64, func(context.Context) error) error
	WithinAdmin(context.Context, func(context.Context) error) error
	AcquireLock(context.Context, string, bool) error
}

// SchoolSettingsStore owns the platform.schools JSONB translation needed by
// the legacy login-image settings. The settings service only sees the value.
type SchoolSettingsStore interface {
	FindSettings(context.Context, int64) (string, error)
	UpdateSettings(context.Context, int64, func(string) (string, error)) error
}

// PayrollCategoryStatus is one Lohnart category with its configured values.
type PayrollCategoryStatus struct {
	// ID is the stable category identifier (e.g. "regelarbeit").
	ID    string `json:"id"`
	Label string `json:"label"`
	// Number is the tenant's Lohnartnummer, empty when not configured.
	Number string `json:"number"`
	// Unit is "stunden" / "tage" / "" and only meaningful when UnitRequired.
	Unit string `json:"unit,omitempty"`
	// UnitRequired marks the categories that carry day values (sick /
	// vacation / training) and therefore need a unit choice.
	UnitRequired bool `json:"unit_required"`
	// SettingKey / UnitSettingKey let the UI write through the settings API.
	SettingKey     string `json:"setting_key"`
	UnitSettingKey string `json:"unit_setting_key,omitempty"`
}

// PayrollStatus is the complete payroll configuration state of a tenant.
type PayrollStatus struct {
	Categories      []PayrollCategoryStatus `json:"categories"`
	Beraternummer   string                  `json:"beraternummer"`
	Mandantennummer string                  `json:"mandantennummer"`
	// LodasHeaderComplete is true when both LODAS header identifiers are
	// set. Lohn und Gehalt does not need them.
	LodasHeaderComplete bool `json:"lodas_header_complete"`
	// ConfiguredCategories counts categories with a Lohnartnummer whose
	// unit requirement (if any) is also satisfied.
	ConfiguredCategories int `json:"configured_categories"`
	TotalCategories      int `json:"total_categories"`
	StaffTotal           int `json:"staff_total"`
	// StaffWithoutPersonnelNumber counts active staff the payroll system
	// could not match. Count only — names stay behind time_tracking:manage.
	StaffWithoutPersonnelNumber int `json:"staff_without_personnel_number"`
	// Dispatch is the Lohnbüro of the school (#4805).
	Dispatch PayrollDispatchSettings `json:"dispatch"`
}

// PayrollDispatchSettings are the values of the automatic send after the
// Monatsabschluss, as /payroll edits them.
type PayrollDispatchSettings struct {
	Auto    bool   `json:"auto"`
	Channel string `json:"channel"`
	Format  string `json:"format"`
	Email   string `json:"email"`
}

// PayrollStatusGetter reports the payroll configuration state.
type PayrollStatusGetter interface {
	GetPayrollStatus(ctx context.Context) (*PayrollStatus, error)
}

type SettingsChangedNotifier func(context.Context, int64, string)

// OperatorValueSetHook runs in the same tenant transaction as an operator
// setting write. The returned closure runs after commit and is meant for
// non-transactional work such as file deletion or broadcasts. Mirrors the
// tenant-side ValueSetCallback contract so side effects apply uniformly
// regardless of who flipped the value.
type OperatorValueSetHook func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)
