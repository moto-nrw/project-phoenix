package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type targets struct {
	capability recordports.OrganizationTargets
}

func (t targets) CountOrganizationsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	count, err := t.capability.CountOrganizationsByID(ctx, ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(count)}
	if errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
		return 0, stats, fmt.Errorf("%w: %v", domain.ErrInvalidTarget, err)
	}
	return count, stats, err
}

func (t targets) CountSchoolsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	count, err := t.capability.CountSchoolsByID(ctx, ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(count)}
	if errors.Is(err, organizationtenancy.ErrInvalidSchool) {
		return 0, stats, fmt.Errorf("%w: %v", domain.ErrInvalidTarget, err)
	}
	return count, stats, err
}

type audit struct{ port recordports.Audit }

func (a audit) Append(ctx context.Context, entry domain.AuditEntry) (domain.OperationStats, error) {
	started := time.Now()
	err := a.port.AppendAnnouncementAudit(ctx, recordports.AuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceID: entry.ResourceID,
		RequestIP: entry.RequestIP, Changes: entry.Changes,
	})
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err == nil {
		stats.Rows = 1
	}
	return stats, err
}

func NewTargets(capability recordports.OrganizationTargets) ports.Targets {
	return targets{capability: capability}
}
func NewAudit(port recordports.Audit) ports.Audit { return audit{port: port} }
