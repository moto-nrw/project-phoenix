package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

const UnregisteredTagScanRetentionDays = 90

type UnregisteredTagScanService interface {
	Record(ctx context.Context, tagUID string, deviceID *int64) error
	ListForOperator(ctx context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error)
	Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error)
	DeleteOlderThan(ctx context.Context, days int) (int, error)
}

type unregisteredTagScanService struct {
	repo        auditModels.UnregisteredTagScanRepository
	command     auditModels.Command
	tenantID    func(context.Context) int64
	withinAdmin func(context.Context, func(context.Context) error) error
}

type UnregisteredTagScanRuntime struct {
	TenantID    func(context.Context) int64
	WithinAdmin func(context.Context, func(context.Context) error) error
}

func NewUnregisteredTagScanService(repo auditModels.UnregisteredTagScanRepository, command auditModels.Command, runtime UnregisteredTagScanRuntime) (UnregisteredTagScanService, error) {
	if repo == nil || command == nil || runtime.TenantID == nil || runtime.WithinAdmin == nil {
		return nil, fmt.Errorf("unregistered tag scan service dependencies are required")
	}
	return &unregisteredTagScanService{repo: repo, command: command, tenantID: runtime.TenantID, withinAdmin: runtime.WithinAdmin}, nil
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
	return s.command.Append(ctx, scan)
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
		scan, err = s.repo.Resolve(adminCtx, id, operatorID, trimPtrToNil(note))
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
	return s.withinAdmin(ctx, fn)
}

func trimPtrToNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
