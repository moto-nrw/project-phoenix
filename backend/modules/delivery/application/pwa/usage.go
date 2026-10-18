// Package pwa collects PWA standalone-usage reporting (#2189): portals
// report "this account uses the app in standalone display mode", the
// operator dashboard and Prometheus read per-school aggregates, and the
// nightly GDPR job sweeps stale rows.
package pwa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// UsageWindowDays / UsageWindow define the reporting window of the
// standalone-usage metric: "used the app in standalone mode within the last
// 30 days".
const (
	UsageWindowDays = 30
	UsageWindow     = UsageWindowDays * 24 * time.Hour
)

// settingPWAUsageRetentionDays is the Settings Platform registry key of the
// retention period. The registry owns it; this package may not import it, so
// TestPWAUsageRetentionKeyMatchesTheRegistry pins the name.
const settingPWAUsageRetentionDays = "gdpr.pwa_usage_retention_days"

// snapshotTTL bounds how often a Prometheus scrape may hit the database.
const snapshotTTL = time.Minute

// CleanupResult summarises one retention run for a single tenant.
type CleanupResult struct {
	RowsDeleted   int
	RetentionDays int
	Cutoff        time.Time
}

// UsageRow is one (school, portal) bucket of PWA standalone-usage counts.
// EligibleUsers counts the accounts of that school matching the portal's
// role predicate; StandaloneUsers never exceeds it.
type UsageRow struct {
	TenantID        int64
	Portal          string
	StandaloneUsers int
	EligibleUsers   int
}

// UsageCounts reads the per-school usage buckets within a window. tenantID 0
// returns every school; callers run it inside an administrative transaction.
type UsageCounts interface {
	PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]UsageRow, error)
}

// RetentionSettings resolves the usage retention window of the current tenant.
type RetentionSettings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

type usageRepository interface {
	RecordSeen(ctx context.Context, tenantID, accountID int64, portal string) error
	DeleteLastSeenBefore(ctx context.Context, tenantID int64, cutoff time.Time) (int, error)
}

// UsageService records and aggregates PWA standalone usage. Report methods
// are idempotent upserts; repeated reports only advance last_seen_at.
type UsageService interface {
	// ReportStaff records standalone usage for a staff-portal session of the
	// current tenant. Runs inside the request's tenant transaction.
	ReportStaff(ctx context.Context, accountID int64) error
	// ReportParent records standalone usage for a parents-portal session,
	// fanning out one row per school the account is actively linked to as a
	// guardian (mirrors push SubscribeParent). Zero mappings is a no-op.
	ReportParent(ctx context.Context, accountID int64) error
	// CleanupExpiredUsage deletes rows of the current tenant whose
	// last_seen_at is older than the gdpr.pwa_usage_retention_days window.
	CleanupExpiredUsage(ctx context.Context) (*CleanupResult, error)
	// SnapshotUsage returns per-school, per-portal counts over UsageWindow
	// for the Prometheus exporter, cached for snapshotTTL so scrapes cannot
	// hammer the database.
	SnapshotUsage() ([]UsageRow, error)
}

type usageService struct {
	db             *bun.DB
	repo           usageRepository
	summaries      UsageCounts
	accountTenants GuardianSchools
	settings       RetentionSettings
	logger         *slog.Logger
	tenantRuntime  *tenant.UnitOfWork

	snapshotMu   sync.Mutex
	snapshot     []UsageRow
	snapshotTime time.Time
}

// NewUsageService builds the PWA usage service.
func NewUsageService(
	db *bun.DB,
	repo usageRepository,
	summaries UsageCounts,
	accountTenants GuardianSchools,
	settings RetentionSettings,
	logger *slog.Logger,
) UsageService {
	if logger == nil {
		logger = slog.Default()
	}
	return &usageService{
		db:             db,
		repo:           repo,
		summaries:      summaries,
		accountTenants: accountTenants,
		settings:       settings,
		logger:         logger,
	}
}

// NewTenantUsageService is NewUsageService over the root's tenant unit of
// work (ADR 0059). A missing collaborator is a construction error; the
// logger stays optional.
func NewTenantUsageService(
	runtime tenant.UnitOfWork,
	db *bun.DB,
	repo usageRepository,
	summaries UsageCounts,
	accountTenants GuardianSchools,
	settings RetentionSettings,
	logger *slog.Logger,
) (UsageService, error) {
	if runtime.IsZero() {
		return nil, fmt.Errorf("pwa usage: %w", tenant.ErrRuntimeRequired)
	}
	if db == nil || repo == nil || summaries == nil || accountTenants == nil || settings == nil {
		return nil, errors.New("pwa usage: all dependencies are required")
	}
	service := NewUsageService(db, repo, summaries, accountTenants, settings, logger).(*usageService)
	service.tenantRuntime = &runtime
	return service, nil
}

func (s *usageService) ReportStaff(ctx context.Context, accountID int64) error {
	if err := validateUsage(accountID, deliveryModels.PushPortalStaff); err != nil {
		return err
	}
	return s.repo.RecordSeen(ctx, tenant.FromContext(ctx), accountID, deliveryModels.PushPortalStaff)
}

func (s *usageService) ReportParent(ctx context.Context, accountID int64) error {
	if err := validateUsage(accountID, deliveryModels.PushPortalParent); err != nil {
		return err
	}
	if s.tenantRuntime != nil {
		ctx = tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
	}
	var mappings []int64
	if err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		mappings, err = s.accountTenants.ListGuardianSchoolIDs(txCtx, accountID)
		if err != nil {
			return fmt.Errorf("resolving guardian tenant mappings: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	// A guardian mid-offboarding simply has nothing to report. Each remaining
	// mapping gets its own tenant role and RLS boundary after the admin read ends.
	for _, mapping := range mappings {
		tenantID, err := tenant.NewTenantID(mapping)
		if err != nil {
			return fmt.Errorf("recording pwa usage for tenant %d: %w", mapping, err)
		}
		if err := tenant.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			if err := validateTenantWriteContext(txCtx, mapping); err != nil {
				return err
			}
			return s.repo.RecordSeen(txCtx, mapping, accountID, deliveryModels.PushPortalParent)
		}); err != nil {
			return fmt.Errorf("recording pwa usage for tenant %d: %w", mapping, err)
		}
	}
	return nil
}

// CleanupExpiredUsage fails closed when settings are unavailable so no
// unverified retention period can delete tenant data.
func (s *usageService) CleanupExpiredUsage(ctx context.Context) (*CleanupResult, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("pwa usage cleanup: no tenant in context")
	}
	if s.settings == nil {
		return nil, fmt.Errorf("pwa usage cleanup: settings service not configured")
	}
	retentionDays, err := s.settings.ResolveInt(ctx, settingPWAUsageRetentionDays)
	if err != nil {
		return nil, fmt.Errorf("resolve pwa usage retention: %w", err)
	}
	if retentionDays <= 0 {
		return nil, fmt.Errorf("pwa usage retention must be positive, got %d", retentionDays)
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted, err := s.repo.DeleteLastSeenBefore(ctx, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete expired pwa usage rows: %w", err)
	}

	if deleted > 0 {
		s.logger.Info("pwa usage cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("rows_deleted", deleted),
			slog.Int("retention_days", retentionDays),
		)
	}
	return &CleanupResult{RowsDeleted: deleted, RetentionDays: retentionDays, Cutoff: cutoff}, nil
}

func validateUsage(accountID int64, portal string) error {
	if accountID <= 0 {
		return errors.New("account_id is required")
	}
	if portal != deliveryModels.PushPortalStaff && portal != deliveryModels.PushPortalParent {
		return errors.New("portal must be 'staff' or 'parent'")
	}
	return nil
}

func validateTenantWriteContext(ctx context.Context, expectedTenantID int64) error {
	if tenant.IsAdminTx(ctx) || tenant.FromContext(ctx) != expectedTenantID {
		return errors.New("pwa usage write requires its tenant transaction")
	}
	return nil
}

func (s *usageService) SnapshotUsage() ([]UsageRow, error) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshot != nil && time.Since(s.snapshotTime) < snapshotTTL {
		return s.snapshot, nil
	}

	var rows []UsageRow
	ctx := context.Background()
	if s.tenantRuntime != nil {
		ctx = tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
	}
	err := tenant.WithAdminTxOrDirect(ctx, s.db, func(adminCtx context.Context) error {
		var qErr error
		rows, qErr = s.summaries.PWAUsage(adminCtx, 0, UsageWindow)
		return qErr
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot pwa usage: %w", err)
	}
	s.snapshot = rows
	s.snapshotTime = time.Now()
	return rows, nil
}
