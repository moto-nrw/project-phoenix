package config

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// SettingsService defines operations for the schema-driven settings system.
// All settings are tenant-scoped with a registry default fallback.
type SettingsService interface {
	// EnrollmentEnabledForTenants is the typed cross-tenant query used by
	// enrollment discovery. Every requested positive tenant is present in the
	// result with either its explicit override or the registered platform
	// default.
	EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error)

	// GetSchema returns the full settings schema with resolved values for the
	// current tenant, filtered by the user's permissions. Excludes settings
	// marked AccessOperatorOnly — they are only visible via GetSchemaForOperator.
	GetSchema(ctx context.Context, userPermissions []string) (*SettingsSchema, error)

	// GetSchemaForOperator returns the full settings schema for platform
	// operators. Excludes AccessAdminOnly settings and skips the per-setting
	// ReadPermission filter (operators see everything they're allowed to see
	// via AccessPolicy). Used by the /api/operator/schools/{id}/settings route.
	GetSchemaForOperator(ctx context.Context, userPermissions []string) (*SettingsSchema, error)

	// Resolve returns the raw value for a setting key (tenant override or registry default).
	Resolve(ctx context.Context, key string) (any, error)

	// ResolveString resolves a setting as a string.
	ResolveString(ctx context.Context, key string) (string, error)

	// ResolveStringForTenant resolves a setting as a string for a specific tenant,
	// handling its own DB transaction context. Used by device auth which runs
	// before the tenant middleware sets PostgreSQL-level tenant context.
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)

	// ResolveStringForTenantInTx resolves a setting as a string on the
	// caller's ALREADY OPEN transaction, bypassing both the context snapshot
	// and the request-scoped memo cache.
	//
	// For callers that must observe committed state as of NOW, from inside a
	// transaction whose locks make that observation meaningful — the
	// school-portal mint guard re-decides the MFA gate this way (#2207).
	// Every other resolve path is deliberately memoized per request, which is
	// exactly wrong there: the login already resolved the same key minutes
	// earlier, so a cached re-read would return the stale value it is meant
	// to replace.
	//
	// Requires an ambient transaction (tenant.WithTransactionForTest) whose role can
	// read config.setting_values for tenantID — phoenix_admin, or a tenant
	// transaction on the same tenant. Without one it returns an error rather
	// than silently falling back to an RLS-filtered read that would report
	// every setting as unset.
	ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error)

	// ResolveBool resolves a setting as a bool.
	ResolveBool(ctx context.Context, key string) (bool, error)

	// ResolveBools resolves multiple boolean settings in one repository query.
	// Every requested key is present in the result, using its registry default
	// when the tenant has no stored override.
	ResolveBools(ctx context.Context, keys []string) (map[string]bool, error)

	// ResolveBoolForTenant resolves a setting as a bool for an explicitly
	// provided tenant — required from call sites that run outside the
	// TenantTxMiddleware (e.g. /auth/mfa/verify, which runs between the
	// challenge handshake and the session being established).
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)

	// ResolveInt resolves a setting as an int.
	ResolveInt(ctx context.Context, key string) (int, error)

	// ResolveIntForTenant resolves a setting as an int for an explicitly
	// provided tenant — mirrors ResolveBoolForTenant for callers that run
	// outside the TenantTxMiddleware (e.g. login flow needs to expose the
	// tenant's trusted-device-days value in the MFA challenge response).
	ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error)

	// HasTenantOverride checks if a tenant has an explicit DB override for a setting.
	HasTenantOverride(ctx context.Context, key string) (bool, error)

	// SetValue sets a tenant override for a setting.
	// userPermissions is checked against the definition's WritePermission.
	// Pass nil to skip permission checks (for system-level callers).
	SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error

	// ResetValue removes a tenant override, falling back to the registry default.
	// userPermissions is checked against the definition's WritePermission.
	// Pass nil to skip permission checks (for system-level callers).
	ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error

	// CheckOperatorWritable enforces the operator write/reveal access policy for
	// a setting key: it returns *DefinitionNotFoundError for an unknown key and
	// ErrOperatorAdminOnly for an AccessAdminOnly setting (which operators must
	// not touch, mirroring GetSchemaForOperator hiding those keys). Returns nil
	// when an operator may write the key.
	CheckOperatorWritable(key string) error

	// GetLoginImageURL returns the loginImageUrl from the school's settings JSONB.
	// Returns empty string if no login image is set.
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)

	// SetLoginImageURL sets the loginImageUrl in the school's settings JSONB.
	// Uses WithAdminTx because platform.schools requires the phoenix_admin role.
	// Returns the previous loginImageUrl (if any) so the caller can clean up the old file.
	SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (oldURL string, err error)

	// ClearLoginImageURL removes the loginImageUrl from the school's settings JSONB.
	// Uses WithAdminTx because platform.schools requires the phoenix_admin role.
	// Returns the previous loginImageUrl (if any) so the caller can clean up the old file.
	ClearLoginImageURL(ctx context.Context, tenantID int64) (oldURL string, err error)

	// LockSlotListCutoffPair takes the per-tenant transaction-scoped advisory lock
	// that guards the Ganztag pickup-cutoff pair. The cutoff writer holds it while
	// it validates the pair; a reader that must observe both cutoffs consistently
	// (services/slotlists) takes it before resolving them, so a concurrent lowering
	// of both boundaries cannot interleave with the read and expose an inverted
	// short/long pair under READ COMMITTED. Best-effort: without an ambient
	// transaction the xact lock is meaningless, so it is skipped rather than failing.
	LockSlotListCutoffPair(ctx context.Context) error

	// LockSlotListCutoffPairShared is the SHARED-mode counterpart taken by read
	// paths (services/slotlists pickupBuckets). Shared holders do not block one
	// another, so concurrent option loads, pickup previews and exports run in
	// parallel instead of serializing behind the exclusive writer lock; it still
	// conflicts with LockSlotListCutoffPair so a cutoff write cannot expose a
	// partial pair to a reader. Same best-effort tx semantics.
	LockSlotListCutoffPairShared(ctx context.Context) error

	// LockClassCollectionPair takes the per-tenant transaction-scoped advisory
	// lock guarding the enrollment class-restriction / class-collection
	// invariant. Both the settings-side class-collection guard and the
	// enrollment-side phase eligibility guard take it on the same key, so a
	// write disabling concrete-class collection and a write activating a
	// class-restricted phase cannot race into a state where every submission
	// fails class_not_eligible. Best-effort: skipped without an ambient tx.
	LockClassCollectionPair(ctx context.Context) error

	// LockMFAPolicy takes the per-tenant transaction-scoped EXCLUSIVE advisory
	// lock guarding security.mfa_mode. SetValue and ResetValue take it before
	// they touch that key, so the write cannot commit while a session mint that
	// already read the old mode is still in flight. Held until the writing
	// transaction commits. Best-effort: skipped without an ambient tx.
	LockMFAPolicy(ctx context.Context) error

	// LockMFAPolicySharedForTenant is the SHARED counterpart taken by the token
	// mint paths that re-decide the MFA gate inside their mint transaction
	// (services/auth schoolMintGuard). Shared holders never block one another,
	// so concurrent logins do not serialize; it conflicts only with the
	// exclusive writer lock above, which is what stops "MFA is off" from being
	// read, an admin enabling MFA, and an MFA-free session being minted
	// afterwards anyway (#2207).
	//
	// The tenant is explicit because the login flows run outside
	// TenantTxMiddleware and carry the resolved school in an argument, not in
	// the context. Same best-effort tx semantics.
	LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error
}

// BatchSettingsService extends SettingsService with query-coalescing reads.
// Keeping it separate preserves the narrow test and domain interfaces that
// intentionally expose only the single-value operations they consume.
type BatchSettingsService interface {
	SettingsService

	ResolveMany(ctx context.Context, keys []string) (*SettingsSnapshot, error)
	ResolveManyForTenant(ctx context.Context, tenantID int64, keys []string) (*SettingsSnapshot, error)
	ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*SettingsSnapshot, error)
}

// --- Response types ---

// SettingsSchema is the top-level response for the schema endpoint.
type SettingsSchema struct {
	Tabs []*SchemaTab `json:"tabs"`
}

// SchemaTab groups categories under a named tab.
type SchemaTab struct {
	Key        string            `json:"key"`
	Label      string            `json:"label"`
	Categories []*SchemaCategory `json:"categories"`
}

// SchemaCategory groups settings under a named category.
type SchemaCategory struct {
	Key   string             `json:"key"`
	Label string             `json:"label"`
	Items []*ResolvedSetting `json:"items"`
}

// ResolvedSetting is a setting definition enriched with its resolved value.
type ResolvedSetting struct {
	Key          string              `json:"key"`
	Label        string              `json:"label"`
	Description  string              `json:"description"`
	Type         config.FieldType    `json:"type"`
	Default      any                 `json:"default"`
	Value        any                 `json:"value"`
	IsDefault    bool                `json:"is_default"`
	Writable     bool                `json:"writable"`
	Visible      bool                `json:"visible"`
	SortOrder    int                 `json:"sort_order"`
	AccessPolicy config.AccessPolicy `json:"access_policy"`

	Validation *config.ValidationRules `json:"validation,omitempty"`
	DependsOn  *config.Dependency      `json:"depends_on,omitempty"`
	Options    *config.SelectOptions   `json:"options,omitempty"`
}
