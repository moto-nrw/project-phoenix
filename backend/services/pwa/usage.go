// Package pwa collects PWA standalone-usage reporting (#2189): portals
// report "this account uses the app in standalone display mode", the
// operator dashboard and Prometheus read per-school aggregates, and the
// nightly GDPR job sweeps stale rows.
package pwa

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/config"
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

// snapshotTTL bounds how often a Prometheus scrape may hit the database.
const snapshotTTL = time.Minute

// CleanupResult summarises one retention run for a single tenant.
type CleanupResult struct {
	RowsDeleted   int
	RetentionDays int
	Cutoff        time.Time
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
	SnapshotUsage() ([]platformModels.SchoolPWAUsageRow, error)
}

type usageService struct {
	db             *bun.DB
	repo           iot.PWAStandaloneUsageRepository
	summaries      platformModels.OperatorSummariesRepository
	accountTenants authModels.AccountTenantRepository
	settings       config.SettingsService
	logger         *slog.Logger
	tenantRuntime  *tenant.Runtime

	snapshotMu   sync.Mutex
	snapshot     []platformModels.SchoolPWAUsageRow
	snapshotTime time.Time
}

func (s *usageService) SetTenantRuntime(runtime tenant.Runtime) {
	s.tenantRuntime = &runtime
}

// NewUsageService builds the PWA usage service.
func NewUsageService(
	db *bun.DB,
	repo iot.PWAStandaloneUsageRepository,
	summaries platformModels.OperatorSummariesRepository,
	accountTenants authModels.AccountTenantRepository,
	settings config.SettingsService,
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

func (s *usageService) ReportStaff(ctx context.Context, accountID int64) error {
	usage := &iot.PWAStandaloneUsage{AccountID: accountID, Portal: iot.PushPortalStaff}
	if err := usage.Validate(); err != nil {
		return err
	}
	return s.repo.RecordSeen(ctx, usage)
}

func (s *usageService) ReportParent(ctx context.Context, accountID int64) error {
	return tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		mappings, err := s.accountTenants.FindActiveGuardianByAccountID(txCtx, accountID)
		if err != nil {
			return fmt.Errorf("resolving guardian tenant mappings: %w", err)
		}
		prototype := iot.PWAStandaloneUsage{AccountID: accountID, Portal: iot.PushPortalParent}
		if err := prototype.Validate(); err != nil {
			return err
		}
		// A guardian mid-offboarding simply has nothing to report — never an
		// error, the report endpoint is fire-and-forget telemetry.
		for _, mapping := range mappings {
			usage := prototype
			usage.SetTenantID(mapping.TenantID)
			if err := s.repo.RecordSeen(txCtx, &usage); err != nil {
				return fmt.Errorf("recording pwa usage for tenant %d: %w", mapping.TenantID, err)
			}
		}
		return nil
	})
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
	retentionDays, err := s.settings.ResolveInt(ctx, configModel.KeyGDPRPWAUsageRetentionDays)
	if err != nil {
		return nil, fmt.Errorf("resolve pwa usage retention: %w", err)
	}
	if retentionDays <= 0 {
		return nil, fmt.Errorf("pwa usage retention must be positive, got %d", retentionDays)
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted, err := s.repo.DeleteLastSeenBefore(ctx, cutoff)
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

func (s *usageService) SnapshotUsage() ([]platformModels.SchoolPWAUsageRow, error) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshot != nil && time.Since(s.snapshotTime) < snapshotTTL {
		return s.snapshot, nil
	}

	var rows []platformModels.SchoolPWAUsageRow
	ctx := context.Background()
	if s.tenantRuntime != nil {
		ctx = tenant.WithRuntime(ctx, *s.tenantRuntime)
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
