package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

const UnregisteredTagScanRetentionDays = 90

// UnregisteredTagScanService records unregistered RFID scans and expires
// them. The operator review reads and resolves them through Device Fleet
// (#3232).
type UnregisteredTagScanService interface {
	Record(ctx context.Context, tagUID string, deviceID *int64) error
	DeleteOlderThan(ctx context.Context, days int) (int, error)
}

type unregisteredTagScanService struct {
	repo     auditModels.UnregisteredTagScanRepository
	tenantID func(context.Context) int64
}

type UnregisteredTagScanRuntime struct {
	TenantID func(context.Context) int64
}

// NewUnregisteredTagScanService builds the retained scan service. repo is
// the Device Fleet owner behind the retained repository port (#2678): every
// scan write, including the append, goes through that owner.
func NewUnregisteredTagScanService(repo auditModels.UnregisteredTagScanRepository, runtime UnregisteredTagScanRuntime) (UnregisteredTagScanService, error) {
	if repo == nil || runtime.TenantID == nil {
		return nil, fmt.Errorf("unregistered tag scan service dependencies are required")
	}
	return &unregisteredTagScanService{repo: repo, tenantID: runtime.TenantID}, nil
}

func (s *unregisteredTagScanService) Record(ctx context.Context, tagUID string, deviceID *int64) error {
	normalized := strings.TrimSpace(tagUID)
	if normalized == "" {
		return fmt.Errorf("tag UID is required")
	}
	tenantID := s.tenantID(ctx)
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

func (s *unregisteredTagScanService) DeleteOlderThan(ctx context.Context, days int) (int, error) {
	if days <= 0 {
		days = UnregisteredTagScanRetentionDays
	}
	return s.repo.DeleteOlderThan(ctx, time.Now().AddDate(0, 0, -days))
}
