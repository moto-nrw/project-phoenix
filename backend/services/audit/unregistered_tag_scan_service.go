package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const UnregisteredTagScanRetentionDays = 90

type UnregisteredTagScanService interface {
	Record(ctx context.Context, tagUID string, deviceID *int64) error
	ListForOperator(ctx context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error)
	Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error)
	DeleteOlderThan(ctx context.Context, days int) (int, error)
}

type unregisteredTagScanService struct {
	repo      auditModels.UnregisteredTagScanRepository
	txHandler *tenant.TransactionRunner
}

func NewUnregisteredTagScanService(repo auditModels.UnregisteredTagScanRepository, db *bun.DB) UnregisteredTagScanService {
	service := &unregisteredTagScanService{repo: repo}
	if db != nil {
		service.txHandler = tenant.NewTransactionRunner()
	}
	return service
}

func (s *unregisteredTagScanService) Record(ctx context.Context, tagUID string, deviceID *int64) error {
	normalized := strings.TrimSpace(tagUID)
	if normalized == "" {
		return fmt.Errorf("tag UID is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return fmt.Errorf("tenant context is required")
	}
	scan := &auditModels.UnregisteredTagScan{
		TagUID:    normalized,
		DeviceID:  deviceID,
		ScannedAt: time.Now(),
	}
	scan.SetTenantID(tenantID)
	return s.repo.Create(ctx, scan)
}

func (s *unregisteredTagScanService) ListForOperator(ctx context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	var scans []*auditModels.UnregisteredTagScan
	err := s.runAdmin(ctx, func(adminCtx context.Context) error {
		var err error
		scans, err = s.repo.ListForOperator(adminCtx, filter)
		return err
	})
	return scans, err
}

func (s *unregisteredTagScanService) Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
	if id <= 0 {
		return nil, fmt.Errorf("scan ID is required")
	}
	if operatorID <= 0 {
		return nil, fmt.Errorf("operator ID is required")
	}
	var scan *auditModels.UnregisteredTagScan
	err := s.runAdmin(ctx, func(adminCtx context.Context) error {
		var err error
		scan, err = s.repo.Resolve(adminCtx, id, operatorID, strutil.TrimPtrToNil(note))
		return err
	})
	return scan, err
}

func (s *unregisteredTagScanService) DeleteOlderThan(ctx context.Context, days int) (int, error) {
	if days <= 0 {
		days = UnregisteredTagScanRetentionDays
	}
	return s.repo.DeleteOlderThan(ctx, time.Now().AddDate(0, 0, -days))
}

func (s *unregisteredTagScanService) runAdmin(ctx context.Context, fn func(context.Context) error) error {
	if s.txHandler == nil {
		return fn(tenant.ContextWithoutTenant(ctx))
	}
	return tenant.WithinAdmin(ctx, fn)
}
